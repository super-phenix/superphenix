package quota

// Hierarchical quotas: an organization quota, split into project quotas, split into AZ quotas
// (one per project and availability zone). Only the AZ quotas are enforced by Kubernetes: the
// quotasync reconciler sends each AZ controller the effective quota of the project in that AZ. The
// organization and project quotas bound what their children can take.
//
// Vocabulary used in this file:
//   - a level is the organization, a project or a project in one AZ;
//   - the effective limit of a level is its own quota, plus 0 for every resource an ancestor
//     limits but its own quota does not (see spxQuota.Effective). A resource absent from an
//     effective limit is not limited at all;
//   - the allocation of a child is what it takes from its parent (see projectAllocations);
//   - an AZ takes the higher of its AZ quota and of the quota it may still enforce: a lowered AZ
//     quota frees capacity once the AZ applies it;
//   - every write runs under a row lock on the organization (see inOrganizationTransaction), so
//     that two concurrent requests cannot both pass the same capacity check.
//
// The rules every write enforces:
//   - a parent can never be set below what its children are allocated (checkNotBelow);
//   - a child can never grow past what its parent has left (checkCapacity);
//   - an AZ quota can never go below what the project uses in the AZ (checkUsage, then the AZ
//     controller on the push).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/az"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	spxQuota "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/quota"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

// ---------------------------------------------------------------------------------------------
// Errors and hooks
// ---------------------------------------------------------------------------------------------

// Business errors of the hierarchical quotas. Every one of them is caused by the request
// (and is safe to return to the caller); any other error is an internal error.
var (
	ErrInvalidQuantity      = spxQuota.ErrInvalidResources
	ErrOrgQuotaExceeded     = errors.New("organization quota exceeded")
	ErrProjectQuotaExceeded = errors.New("project quota exceeded")
	ErrOrganizationNotFound = errors.New("organization not found")
	ErrProjectNotFound      = errors.New("project not found")
	ErrAZNotFound           = errors.New("availability zone not found")
	ErrProjectQuotaEnforced = errors.New("the project quota cannot be removed while the organization has a quota")
	ErrBelowAllocated       = errors.New("quota below what is already allocated")
	ErrBelowUsage           = errors.New("quota below what is currently used")
	ErrUsageUnavailable     = errors.New("quota usage unavailable, retry later")
)

// ReadUsage asks the controller of one AZ what a project uses there (the `status.used` of its
// ResourceQuota). The quotasync package sets it. While it is nil, SetProjectAZQuota fails when it
// lowers a limit.
var ReadUsage func(ctx context.Context, orgId, projectId uuid.UUID, codeAZ string) (spxQuota.Quantities, error)

// ---------------------------------------------------------------------------------------------
// Overviews (read only, returned by the API)
// ---------------------------------------------------------------------------------------------

// OrganizationQuotaOverview is the quota of an organization (Limit, absent when Unlimited) and
// its distribution to projects: what they are allocated (see projectAllocations, 0 for a limited
// resource no project holds) and what remains. ProjectQuotas holds the effective limit of the
// projects (see spxQuota.Effective): when the organization has a quota, every live project is
// listed, a resource the organization limits but the project quota does not being limited to 0.
type OrganizationQuotaOverview struct {
	OrganizationId string               `json:"organizationId"`
	Unlimited      bool                 `json:"unlimited"`
	Limit          model.QuotaResources `json:"limit,omitempty"`
	Allocated      model.QuotaResources `json:"allocated"`
	Remaining      model.QuotaResources `json:"remaining,omitempty"`
	ProjectQuotas  []model.ProjectQuota `json:"projectQuotas"`
}

// ProjectQuotaOverview is the effective quota of a project (Limit, absent when Unlimited: the
// project quota, plus 0 for every resource the organization limits but the project quota does
// not) and its distribution to AZs: what they are allocated (the sum of the AZ quotas, 0 for a
// limited resource no AZ holds) and what remains.
type ProjectQuotaOverview struct {
	ProjectId      string               `json:"projectId"`
	OrganizationId string               `json:"organizationId"`
	Unlimited      bool                 `json:"unlimited"`
	Limit          model.QuotaResources `json:"limit,omitempty"`
	Allocated      model.QuotaResources `json:"allocated"`
	Remaining      model.QuotaResources `json:"remaining,omitempty"`
	AZQuotas       []AZQuotaOverview    `json:"azQuotas"`
}

// AZQuotaOverview is the effective quota of a project in one AZ (Limit, the one enforced, see
// Hierarchy.AZ; absent when no level limits anything) and what the project currently uses there
// (Used, the `status.used` of its ResourceQuota, filled by the quotasync package). Kubernetes
// only tracks the resources the ResourceQuota limits, so a resource without limit in the AZ is
// absent from Used.
type AZQuotaOverview struct {
	CodeAZ string               `json:"codeAZ"`
	Limit  model.QuotaResources `json:"limit,omitempty"`
	// Status tells whether the AZ applied Limit: applied, pending, or error.
	Status string `json:"status" enums:"applied,pending,error"`
	// Error is the error of the last push to the AZ when Status is error.
	Error string `json:"error,omitempty"`
	// Available is false when the usage could not be read from the AZ: Used is then empty.
	Available bool                 `json:"available"`
	Used      model.QuotaResources `json:"used"`
}

// Sync statuses of the quota of a project in one AZ.
const (
	StatusApplied = "applied"
	StatusPending = "pending"
	StatusError   = "error"
)

// Status returns the sync status of a state, and the error of its last push.
func Status(state model.ProjectAZQuotaState) (string, string) {
	switch {
	case state.LastError != nil:
		return StatusError, *state.LastError
	case state.WantedRevision > 0 && state.AppliedRevision >= state.WantedRevision:
		return StatusApplied, ""
	default:
		return StatusPending, ""
	}
}

// GetOrganizationQuotaOverview returns the organization quota (unlimited when not defined or
// empty), what its projects are allocated (see projectAllocations) and what remains.
func GetOrganizationQuotaOverview(ctx context.Context, organizationId uuid.UUID) (OrganizationQuotaOverview, error) {
	database := db.Client.WithContext(ctx)
	overview := OrganizationQuotaOverview{OrganizationId: organizationId.String(), Unlimited: true}

	organizationQuota, err := findOrganizationQuota(database, organizationId)
	if err != nil {
		return overview, err
	}
	projectQuotas, allocations, err := projectAllocations(database, organizationId)
	if err != nil {
		return overview, err
	}
	allocated := sumAllocations(allocations)
	overview.ProjectQuotas = projectQuotas
	overview.Allocated = allocated.Resources()

	// A quota that limits no resource (e.g. set to {}) is unlimited too.
	if organizationQuota == nil || len(organizationQuota.Resources) == 0 {
		return overview, nil
	}
	organizationLimit, err := organizationQuota.Resources.Parse()
	if err != nil {
		return overview, err
	}
	overview.Unlimited = false
	overview.Limit = organizationQuota.Resources
	overview.Allocated = allocated.WithZeros(organizationLimit).Resources()
	overview.Remaining = organizationLimit.Sub(allocated).Resources()
	overview.ProjectQuotas, err = effectiveProjectQuotas(database, organizationId, organizationQuota.Resources, projectQuotas)
	return overview, err
}

// GetProjectQuotaOverview returns the effective quota of a project of the organization
// (unlimited when no level limits anything), the effective quota of each of its AZs, what they
// are allocated and what remains. The usage of the AZs is not read here (see
// quotasync.GetProjectQuotaOverview).
func GetProjectQuotaOverview(ctx context.Context, organizationId, projectId uuid.UUID) (ProjectQuotaOverview, error) {
	database := db.Client.WithContext(ctx)
	overview := ProjectQuotaOverview{
		ProjectId:      projectId.String(),
		OrganizationId: organizationId.String(),
		Unlimited:      true,
		AZQuotas:       []AZQuotaOverview{},
	}

	if err := checkProject(database, organizationId, projectId); err != nil {
		return overview, err
	}
	hierarchy, err := loadHierarchy(database, organizationId, projectId)
	if err != nil {
		return overview, err
	}
	allocatedToAZs, err := sumAZQuotas(hierarchy, "")
	if err != nil {
		return overview, err
	}
	overview.AZQuotas = azQuotaOverviews(organizationId, hierarchy)
	overview.Allocated = allocatedToAZs.Resources()

	effectiveLimit := projectLimit(hierarchy.Organization, hierarchy.Project)
	// No level limits anything (no quota, or quotas set to {}): unlimited.
	if len(effectiveLimit) == 0 {
		return overview, nil
	}
	parsedLimit, err := effectiveLimit.Parse()
	if err != nil {
		return overview, err
	}
	overview.Unlimited = false
	overview.Limit = effectiveLimit
	overview.Allocated = allocatedToAZs.WithZeros(parsedLimit).Resources()
	overview.Remaining = parsedLimit.Sub(allocatedToAZs).Resources()
	return overview, nil
}

// azQuotaOverviews lists the effective quota of the project in every AZ of the organization,
// plus the AZs the organization no longer has but that still hold a quota, sorted by code. Used
// stays empty for the quotasync package to fill.
func azQuotaOverviews(organizationId uuid.UUID, hierarchy Hierarchy) []AZQuotaOverview {
	codes := hierarchy.allocatedCodes()
	for _, code := range azCodes(organizationId) {
		if !slices.Contains(codes, code) {
			codes = append(codes, code)
		}
	}
	slices.Sort(codes)

	overviews := make([]AZQuotaOverview, 0, len(codes))
	for _, code := range codes {
		effectiveLimit := hierarchy.AZ(code)
		if len(effectiveLimit) == 0 {
			// Nothing is limited in this AZ: the limit is omitted from the response.
			effectiveLimit = nil
		}
		state, synced := hierarchy.States[code]
		status, syncError := StatusPending, ""
		if synced {
			status, syncError = Status(state)
		}
		overviews = append(overviews, AZQuotaOverview{
			CodeAZ: code, Limit: effectiveLimit, Status: status, Error: syncError, Used: model.QuotaResources{},
		})
	}
	return overviews
}

// effectiveProjectQuotas lists every live project of the organization with its effective limit
// (see projectLimit): a project without project quota is listed too, limited to 0 on every
// resource the organization limits.
func effectiveProjectQuotas(database *gorm.DB, organizationId uuid.UUID, organizationResources model.QuotaResources, projectQuotas []model.ProjectQuota) ([]model.ProjectQuota, error) {
	projectIds, err := listProjectIds(database, organizationId)
	if err != nil {
		return nil, err
	}
	definedQuotas := quotasByProject(projectQuotas)

	effectiveQuotas := make([]model.ProjectQuota, 0, len(projectIds))
	for _, projectId := range projectIds {
		effectiveQuotas = append(effectiveQuotas, model.ProjectQuota{
			ProjectId:      projectId,
			OrganizationId: organizationId,
			Resources:      projectLimit(organizationResources, definedQuotas[projectId]),
		})
	}
	return effectiveQuotas, nil
}

// ---------------------------------------------------------------------------------------------
// Hierarchy of a project
// ---------------------------------------------------------------------------------------------

// Hierarchy holds the quotas of a project and its sync states. A nil level has no quota.
type Hierarchy struct {
	Organization model.QuotaResources
	Project      model.QuotaResources
	// AZs maps the code of an AZ to the AZ quota of the project there (only the AZs with one).
	AZs map[string]model.QuotaResources
	// States maps the code of an AZ to the sync state of the project there (only the AZs with one).
	States map[string]model.ProjectAZQuotaState
}

// newHierarchy returns an empty Hierarchy.
func newHierarchy() Hierarchy {
	return Hierarchy{AZs: map[string]model.QuotaResources{}, States: map[string]model.ProjectAZQuotaState{}}
}

// AZ returns the effective quota of the project in the given AZ, i.e. what Kubernetes enforces
// there: the AZ quota, plus 0 for every resource the organization or project quota limits but
// the AZ quota does not.
func (hierarchy Hierarchy) AZ(codeAZ string) model.QuotaResources {
	return spxQuota.EffectiveAZQuota(hierarchy.Organization, hierarchy.Project, hierarchy.AZs[codeAZ])
}

// allocatedAZ returns what the project is allocated in the given AZ.
func (hierarchy Hierarchy) allocatedAZ(codeAZ string) (spxQuota.Quantities, error) {
	return azAllocation(hierarchy.AZs[codeAZ], hierarchy.States[codeAZ].Held)
}

// allocatedCodes returns, sorted, the AZs where the project is allocated something: those with an
// AZ quota or a held quota.
func (hierarchy Hierarchy) allocatedCodes() []string {
	codes := slices.Collect(maps.Keys(hierarchy.AZs))
	for code, state := range hierarchy.States {
		if _, hasAZQuota := hierarchy.AZs[code]; !hasAZQuota && len(state.Held) > 0 {
			codes = append(codes, code)
		}
	}
	slices.Sort(codes)
	return codes
}

// azAllocation returns what an AZ takes: per resource, the higher of its AZ quota and of held, the
// quota the AZ may still enforce.
func azAllocation(desired, held model.QuotaResources) (spxQuota.Quantities, error) {
	allocated, err := desired.Parse()
	if err != nil || len(held) == 0 {
		return allocated, err
	}
	parsedHeld, err := held.Parse()
	if err != nil {
		return nil, err
	}
	return allocated.Max(parsedHeld), nil
}

// GetHierarchy reads the organization, project and AZ quotas of a project and its sync states
// from one snapshot.
func GetHierarchy(ctx context.Context, organizationId, projectId uuid.UUID) (Hierarchy, error) {
	var hierarchy Hierarchy
	err := inSnapshot(ctx, func(transaction *gorm.DB) error {
		var err error
		hierarchy, err = loadHierarchy(transaction, organizationId, projectId)
		return err
	})
	return hierarchy, err
}

// loadHierarchy reads, in this order, the organization quota, the project quota, the AZ quotas
// and the sync states of a project.
func loadHierarchy(database *gorm.DB, organizationId, projectId uuid.UUID) (Hierarchy, error) {
	hierarchy := newHierarchy()

	organizationQuota, err := findOrganizationQuota(database, organizationId)
	if err != nil {
		return hierarchy, err
	}
	if organizationQuota != nil {
		hierarchy.Organization = organizationQuota.Resources
	}

	projectQuota, err := findProjectQuota(database, projectId)
	if err != nil {
		return hierarchy, err
	}
	if projectQuota != nil {
		hierarchy.Project = projectQuota.Resources
	}

	azQuotas, err := listAZQuotas(database, projectId)
	if err != nil {
		return hierarchy, err
	}
	for _, azQuota := range azQuotas {
		hierarchy.AZs[azQuota.CodeAZ] = azQuota.Resources
	}

	states, err := listStates(database, projectId)
	if err != nil {
		return hierarchy, err
	}
	for _, state := range states {
		hierarchy.States[state.CodeAZ] = state
	}
	return hierarchy, nil
}

// projectLimit returns the effective limit of a project, the one its AZ quotas must fit in: its
// project quota, plus 0 for every resource the organization limits but the project quota does
// not. The project gets nothing of such a resource, so its AZ quotas cannot escape the
// organization quota. Either quota can be nil; the result is empty when nothing is limited.
func projectLimit(organizationResources, projectResources model.QuotaResources) model.QuotaResources {
	return spxQuota.Effective(projectResources, organizationResources)
}

// ---------------------------------------------------------------------------------------------
// Other reads
// ---------------------------------------------------------------------------------------------

// ListAllProjects lists the live projects of every organization (only their ID and
// organization ID), ordered by organization then project.
func ListAllProjects(ctx context.Context) ([]model.Project, error) {
	var projects []model.Project
	err := db.Client.WithContext(ctx).Select("id", "orga_id").Order("orga_id").Order("id").Find(&projects).Error
	return projects, err
}

// GetProjectQuota retrieves a project's quota, or nil if not defined.
func GetProjectQuota(ctx context.Context, projectId uuid.UUID) (*model.ProjectQuota, error) {
	return findProjectQuota(db.Client.WithContext(ctx), projectId)
}

// ---------------------------------------------------------------------------------------------
// Writes
// ---------------------------------------------------------------------------------------------

// SetOrganizationQuota creates or updates an organization quota (platform administrators only).
//
// The organization quota cannot be lower than what its projects are allocated (see
// projectAllocations), on any resource it limits: ErrBelowAllocated otherwise. The projects
// must be lowered first.
//
// When no project quota exists yet, the organization quota is distributed among its projects
// (see distributeAmongProjects). Otherwise, the existing project quotas are completed so that no
// project ends up below its AZ quotas (see coverAZQuotas).
func SetOrganizationQuota(ctx context.Context, organizationId uuid.UUID, resources model.QuotaResources) (*model.OrganizationQuota, error) {
	organizationLimit, err := resources.Parse()
	if err != nil {
		return nil, err
	}

	saved := model.OrganizationQuota{OrganizationId: organizationId, Resources: resources}
	err = inQuotaWrite(ctx, organizationId, func(transaction *gorm.DB, revision int64) error {
		projectQuotas, allocations, err := projectAllocations(transaction, organizationId)
		if err != nil {
			return err
		}
		if err := checkNotBelow(organizationLimit, sumAllocations(allocations), "its projects"); err != nil {
			return err
		}

		if len(projectQuotas) == 0 {
			err = distributeAmongProjects(transaction, organizationId, organizationLimit, allocations)
		} else {
			err = coverAZQuotas(transaction, organizationId, organizationLimit, projectQuotas, allocations)
		}
		if err != nil {
			return err
		}

		if err := upsert(transaction, &saved, []string{"organization_id"}, "resources"); err != nil {
			return err
		}
		return markOrganizationDirty(transaction, organizationId, revision)
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

// SetProjectQuota sets the quota of a project of the organization. When the project has no
// AZ quota yet, it is split equally among the AZs of the organization; otherwise the AZ quotas
// are kept as they are.
//
// The effective project limit (see projectLimit) cannot be lower than the sum of its AZ quotas
// (see checkProjectCoversAZs). When the organization has a quota, the project must also fit in
// what the organization has left (see checkProjectFitsInOrganization).
func SetProjectQuota(ctx context.Context, organizationId, projectId uuid.UUID, resources model.QuotaResources) (*model.ProjectQuota, error) {
	requested, err := resources.Parse()
	if err != nil {
		return nil, err
	}

	var saved model.ProjectQuota
	err = inProjectTransaction(ctx, organizationId, projectId, func(transaction *gorm.DB, revision int64) error {
		hierarchy, err := loadHierarchy(transaction, organizationId, projectId)
		if err != nil {
			return err
		}
		if err := checkProjectCoversAZs(hierarchy, resources); err != nil {
			return err
		}
		if err := checkProjectFitsInOrganization(transaction, organizationId, projectId, requested, hierarchy.Organization); err != nil {
			return err
		}

		hasAZQuotas := len(hierarchy.AZs) > 0
		saved, err = saveProjectQuota(transaction, organizationId, projectId, resources, hasAZQuotas)
		if err != nil {
			return err
		}
		return markDirty(transaction, organizationId, revision, []uuid.UUID{projectId}, azCodes(organizationId))
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

// SetProjectAZQuota sets the quota of a project in one AZ, whether or not the project has a
// quota. The sum of the AZ quotas must fit in the effective project limit (see
// checkAZFitsInProject).
//
// A limit lowered in the AZ cannot go below what the project uses there: ErrBelowUsage, or
// ErrUsageUnavailable when the AZ cannot report it. Other AZs and projects get the capacity of a
// lowered AZ quota once the AZ applies it.
func SetProjectAZQuota(ctx context.Context, organizationId, projectId uuid.UUID, codeAZ string, resources model.QuotaResources) (*model.ProjectAZQuota, error) {
	requested, err := resources.Parse()
	if err != nil {
		return nil, err
	}
	if _, err := az.GetByCode(codeAZ, organizationId.String()); err != nil {
		return nil, ErrAZNotFound
	}
	// Check the project before asking its AZ for the usage.
	if err := checkProject(db.Client.WithContext(ctx), organizationId, projectId); err != nil {
		return nil, err
	}
	current, err := GetHierarchy(ctx, organizationId, projectId)
	if err != nil {
		return nil, err
	}
	lowered, err := loweredLimits(current.AZ(codeAZ), spxQuota.EffectiveAZQuota(current.Organization, current.Project, resources))
	if err != nil {
		return nil, err
	}
	if err := checkUsage(ctx, organizationId, projectId, codeAZ, lowered); err != nil {
		return nil, err
	}

	var saved model.ProjectAZQuota
	err = inProjectTransaction(ctx, organizationId, projectId, func(transaction *gorm.DB, revision int64) error {
		hierarchy, err := loadHierarchy(transaction, organizationId, projectId)
		if err != nil {
			return err
		}
		if err := checkAZFitsInProject(hierarchy, codeAZ, requested); err != nil {
			return err
		}
		saved, err = upsertAZQuota(transaction, projectId, codeAZ, resources)
		if err != nil {
			return err
		}
		return markDirty(transaction, organizationId, revision, []uuid.UUID{projectId}, []string{codeAZ})
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

// DeleteOrganizationQuota removes an organization quota (platform administrators only).
// The project quotas are kept and keep being enforced.
func DeleteOrganizationQuota(ctx context.Context, organizationId uuid.UUID) error {
	return inQuotaWrite(ctx, organizationId, func(transaction *gorm.DB, revision int64) error {
		err := transaction.Unscoped().Where("organization_id = ?", organizationId).Delete(&model.OrganizationQuota{}).Error
		if err != nil {
			return err
		}
		return markOrganizationDirty(transaction, organizationId, revision)
	})
}

// DeleteProjectQuota removes the quota of a project of the organization and its AZ quotas.
// It returns ErrProjectQuotaEnforced while the organization has a quota. It keeps the sync states
// until the AZs apply the change.
func DeleteProjectQuota(ctx context.Context, organizationId, projectId uuid.UUID) error {
	return inProjectTransaction(ctx, organizationId, projectId, func(transaction *gorm.DB, revision int64) error {
		organizationQuota, err := findOrganizationQuota(transaction, organizationId)
		if err != nil {
			return err
		}
		if organizationQuota != nil {
			return ErrProjectQuotaEnforced
		}
		if err := deleteProjectRows(transaction, projectId); err != nil {
			return err
		}
		return markDirty(transaction, organizationId, revision, []uuid.UUID{projectId}, azCodes(organizationId))
	})
}

// DeleteForProject removes, within transaction, the quota rows and sync states of a project being
// deleted, under the organization lock. The garbage collector deletes the AZ ResourceQuotas.
func DeleteForProject(transaction *gorm.DB, organizationId, projectId uuid.UUID) error {
	if err := lockOrganization(transaction, organizationId); err != nil {
		return err
	}
	if err := deleteProjectRows(transaction, projectId); err != nil {
		return err
	}
	return transaction.Where("project_id = ?", projectId).Delete(&model.ProjectAZQuotaState{}).Error
}

// DeleteForOrganization removes, within transaction, the quota rows of an organization being
// deleted: its organization quota and the project and AZ quotas of all its projects (deleted ones
// included), and their sync states, under the organization lock. The garbage collector deletes
// the AZ ResourceQuotas.
func DeleteForOrganization(transaction *gorm.DB, organizationId uuid.UUID) error {
	if err := lockOrganization(transaction, organizationId); err != nil {
		return err
	}
	if err := transaction.Where("organization_id = ?", organizationId).Delete(&model.ProjectAZQuotaState{}).Error; err != nil {
		return err
	}
	allProjectIds := transaction.Session(&gorm.Session{NewDB: true}).Unscoped().
		Model(&model.Project{}).Select("id").Where("orga_id = ?", organizationId)
	if err := transaction.Unscoped().Where("project_id IN (?)", allProjectIds).Delete(&model.ProjectAZQuota{}).Error; err != nil {
		return err
	}
	if err := transaction.Unscoped().Where("organization_id = ?", organizationId).Delete(&model.ProjectQuota{}).Error; err != nil {
		return err
	}
	return transaction.Unscoped().Where("organization_id = ?", organizationId).Delete(&model.OrganizationQuota{}).Error
}

// deleteProjectRows hard-deletes the AZ quotas and the project quota of a project, so that
// setting them again later is a plain insert.
func deleteProjectRows(transaction *gorm.DB, projectId uuid.UUID) error {
	if err := transaction.Unscoped().Where("project_id = ?", projectId).Delete(&model.ProjectAZQuota{}).Error; err != nil {
		return err
	}
	return transaction.Unscoped().Where("project_id = ?", projectId).Delete(&model.ProjectQuota{}).Error
}

// ---------------------------------------------------------------------------------------------
// Distribution of a parent quota to its children
// ---------------------------------------------------------------------------------------------

// distributeAmongProjects gives a project quota to every live project of an organization that
// has none yet: what its AZ quotas already hold (its allocation), plus an equal share of what the
// organization has left. Only the resources the organization limits are given. A project without
// AZ quota gets its project quota split among the AZs (see saveProjectQuota).
func distributeAmongProjects(transaction *gorm.DB, organizationId uuid.UUID, organizationLimit spxQuota.Quantities, allocations map[uuid.UUID]spxQuota.Quantities) error {
	projectIds, err := listProjectIds(transaction, organizationId)
	if err != nil {
		return err
	}
	azQuotas, err := listOrganizationAZQuotas(transaction, organizationId)
	if err != nil {
		return err
	}
	withAZQuotas := map[uuid.UUID]bool{}
	for _, azQuota := range azQuotas {
		withAZQuotas[azQuota.ProjectId] = true
	}
	equalShare := organizationLimit.Sub(sumAllocations(allocations)).SplitEqually(len(projectIds))

	for _, projectId := range projectIds {
		// Without a project quota, a project takes what its AZs hold.
		projectQuota := equalShare.Add(allocations[projectId].Only(organizationLimit))
		if _, err := saveProjectQuota(transaction, organizationId, projectId, projectQuota.Resources(), withAZQuotas[projectId]); err != nil {
			return err
		}
	}
	return nil
}

// coverAZQuotas adds to the project quotas, for every resource the organization limits and a
// project quota does not, what the AZ quotas of the project hold of it (its allocation, see
// projectAllocations). Such a resource would otherwise count as 0 for the project (see
// projectLimit), below its AZ quotas. A project with AZ quotas but no project quota gets one.
func coverAZQuotas(transaction *gorm.DB, organizationId uuid.UUID, organizationLimit spxQuota.Quantities, projectQuotas []model.ProjectQuota, allocations map[uuid.UUID]spxQuota.Quantities) error {
	definedQuotas := quotasByProject(projectQuotas)

	// Sorted so that the writes always happen in the same order.
	projectIds := slices.SortedFunc(maps.Keys(allocations), func(first, second uuid.UUID) int {
		return strings.Compare(first.String(), second.String())
	})
	for _, projectId := range projectIds {
		missing := spxQuota.Quantities{}
		for name, held := range allocations[projectId].Only(organizationLimit) {
			if _, limitedByProject := definedQuotas[projectId][name]; !limitedByProject && held > 0 {
				missing[name] = held
			}
		}
		if len(missing) == 0 {
			continue
		}

		completed := model.QuotaResources{}
		maps.Copy(completed, definedQuotas[projectId])
		maps.Copy(completed, missing.Resources())
		if _, err := upsertProjectQuota(transaction, organizationId, projectId, completed); err != nil {
			return err
		}
	}
	return nil
}

// saveProjectQuota writes a project quota. When the project has no AZ quota yet, the quota is
// also split equally among the AZs of the organization; AZ quotas already defined are never
// overwritten, they are the project administrator's distribution.
func saveProjectQuota(transaction *gorm.DB, organizationId, projectId uuid.UUID, resources model.QuotaResources, hasAZQuotas bool) (model.ProjectQuota, error) {
	projectQuota, err := upsertProjectQuota(transaction, organizationId, projectId, resources)
	if err != nil || hasAZQuotas {
		return projectQuota, err
	}
	return projectQuota, splitAmongAZs(transaction, organizationId, projectId, resources)
}

// splitAmongAZs gives every AZ of the organization an equal share of the project quota.
func splitAmongAZs(transaction *gorm.DB, organizationId, projectId uuid.UUID, projectResources model.QuotaResources) error {
	parsedProjectQuota, err := projectResources.Parse()
	if err != nil {
		return err
	}
	azConfigs := az.FindAll(organizationId.String())
	slices.SortFunc(azConfigs, func(first, second config.AZConfig) int { return strings.Compare(first.Code, second.Code) })

	equalShare := parsedProjectQuota.SplitEqually(len(azConfigs)).Resources()
	for _, azConfig := range azConfigs {
		if _, err := upsertAZQuota(transaction, projectId, azConfig.Code, equalShare); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------------------------
// Allocations: what the children take from their parent
// ---------------------------------------------------------------------------------------------

// projectAllocations returns the project quotas of the live projects of the organization and
// what each of these projects takes from the organization quota: per resource, the highest of
// its project quota and the sum of its AZ quotas (what the AZs enforce). A project without project
// quota but with AZ quotas is allocated the sum of its AZ quotas.
//
// AZ quotas set while the organization had no quota therefore keep counting once it gets one,
// and lowering a project quota below its AZ quotas frees nothing for the other projects until
// the AZ quotas are lowered too.
func projectAllocations(database *gorm.DB, organizationId uuid.UUID) ([]model.ProjectQuota, map[uuid.UUID]spxQuota.Quantities, error) {
	projectQuotas, err := listProjectQuotas(database, organizationId)
	if err != nil {
		return nil, nil, err
	}
	allocations := make(map[uuid.UUID]spxQuota.Quantities, len(projectQuotas))
	for _, projectQuota := range projectQuotas {
		parsed, err := projectQuota.Resources.Parse()
		if err != nil {
			return nil, nil, err
		}
		allocations[projectQuota.ProjectId] = parsed
	}

	heldByAZs, err := azAllocations(database, organizationId)
	if err != nil {
		return nil, nil, err
	}
	for projectId, held := range heldByAZs {
		allocations[projectId] = allocations[projectId].Max(held)
	}
	return projectQuotas, allocations, nil
}

// azKey identifies the quota of a project in one AZ.
type azKey struct {
	projectId uuid.UUID
	codeAZ    string
}

// azAllocations returns, for every live project of the organization with an AZ quota or a held
// quota, the sum of what its AZs are allocated.
func azAllocations(database *gorm.DB, organizationId uuid.UUID) (map[uuid.UUID]spxQuota.Quantities, error) {
	azQuotas, err := listOrganizationAZQuotas(database, organizationId)
	if err != nil {
		return nil, err
	}
	states, err := listOrganizationStates(database, organizationId)
	if err != nil {
		return nil, err
	}
	held := map[azKey]model.QuotaResources{}
	for _, state := range states {
		if len(state.Held) > 0 {
			held[azKey{state.ProjectId, state.CodeAZ}] = state.Held
		}
	}

	allocations := map[uuid.UUID]spxQuota.Quantities{}
	add := func(projectId uuid.UUID, desired, held model.QuotaResources) error {
		parsed, err := azAllocation(desired, held)
		if err == nil {
			allocations[projectId] = allocations[projectId].Add(parsed)
		}
		return err
	}
	for _, azQuota := range azQuotas {
		key := azKey{azQuota.ProjectId, azQuota.CodeAZ}
		if err := add(azQuota.ProjectId, azQuota.Resources, held[key]); err != nil {
			return nil, err
		}
		delete(held, key)
	}
	for key, resources := range held {
		if err := add(key.projectId, nil, resources); err != nil {
			return nil, err
		}
	}
	return allocations, nil
}

// sumAllocations returns the total of the allocations of the projects.
func sumAllocations(allocations map[uuid.UUID]spxQuota.Quantities) spxQuota.Quantities {
	total := spxQuota.Quantities{}
	for _, allocation := range allocations {
		total = total.Add(allocation)
	}
	return total
}

// sumAZQuotas returns the total the AZs of a project take from it, leaving out excludedAZ when
// not empty.
func sumAZQuotas(hierarchy Hierarchy, excludedAZ string) (spxQuota.Quantities, error) {
	total := spxQuota.Quantities{}
	for _, codeAZ := range hierarchy.allocatedCodes() {
		if codeAZ == excludedAZ {
			continue
		}
		parsed, err := hierarchy.allocatedAZ(codeAZ)
		if err != nil {
			return nil, err
		}
		total = total.Add(parsed)
	}
	return total, nil
}

// quotasByProject indexes project quotas by project ID.
func quotasByProject(projectQuotas []model.ProjectQuota) map[uuid.UUID]model.QuotaResources {
	byProject := make(map[uuid.UUID]model.QuotaResources, len(projectQuotas))
	for _, projectQuota := range projectQuotas {
		byProject[projectQuota.ProjectId] = projectQuota.Resources
	}
	return byProject
}

// ---------------------------------------------------------------------------------------------
// Rules
// ---------------------------------------------------------------------------------------------

// checkProjectCoversAZs checks that the effective limit of a project with the requested project
// quota (see projectLimit) is not below the sum of its AZ quotas. Omitting a resource the
// organization limits counts as limiting it to 0, so it is refused while the AZ quotas hold some.
func checkProjectCoversAZs(hierarchy Hierarchy, requested model.QuotaResources) error {
	effectiveLimit, err := projectLimit(hierarchy.Organization, requested).Parse()
	if err != nil {
		return err
	}
	allocatedToAZs, err := sumAZQuotas(hierarchy, "")
	if err != nil {
		return err
	}
	err = checkNotBelow(effectiveLimit, allocatedToAZs, "its availability zones")
	if err != nil && len(hierarchy.Organization) > 0 {
		return fmt.Errorf("%w (a resource the organization limits counts as 0 when the project quota omits it)", err)
	}
	return err
}

// checkProjectFitsInOrganization checks, when the organization has a quota, that the requested
// project quota fits in what the organization has left once the other projects are allocated.
func checkProjectFitsInOrganization(transaction *gorm.DB, organizationId, projectId uuid.UUID, requested spxQuota.Quantities, organizationResources model.QuotaResources) error {
	if len(organizationResources) == 0 {
		return nil
	}
	organizationLimit, err := organizationResources.Parse()
	if err != nil {
		return err
	}
	_, allocations, err := projectAllocations(transaction, organizationId)
	if err != nil {
		return err
	}
	current := allocations[projectId]
	delete(allocations, projectId)
	return checkCapacity(requested, current, sumAllocations(allocations), organizationLimit, ErrOrgQuotaExceeded)
}

// checkAZFitsInProject checks that the requested AZ quota, added to the other AZ quotas of the
// project, fits in the effective project limit (see projectLimit).
func checkAZFitsInProject(hierarchy Hierarchy, codeAZ string, requested spxQuota.Quantities) error {
	effectiveLimit, err := projectLimit(hierarchy.Organization, hierarchy.Project).Parse()
	if err != nil {
		return err
	}
	current, err := hierarchy.allocatedAZ(codeAZ)
	if err != nil {
		return err
	}
	allocatedToOtherAZs, err := sumAZQuotas(hierarchy, codeAZ)
	if err != nil {
		return err
	}
	return checkCapacity(requested, current, allocatedToOtherAZs, effectiveLimit, ErrProjectQuotaExceeded)
}

// checkCapacity checks that a child quota (requested, replacing current) fits in the parent
// limit, allocatedToOthers being the sum of the other children. Resources the parent does not
// limit are free.
//
// No write can make the children exceed their parent: a parent is never set below its children
// (see checkNotBelow) and a child never grows past its parent. Only the resources the request
// increases are checked, which is the same as long as the children fit; it only lets a state
// stored before these rules (children above their parent) be lowered until it fits again, and
// keeps a resource overcommitted by the other children from blocking a request that does not
// increase it.
func checkCapacity(requested, current, allocatedToOthers, limit spxQuota.Quantities, errExceeded error) error {
	increased, limitOfIncreased := spxQuota.Quantities{}, spxQuota.Quantities{}
	for name, requestedValue := range requested {
		limitValue, limited := limit[name]
		if limited && requestedValue > current[name] {
			increased[name] = requestedValue
			limitOfIncreased[name] = limitValue
		}
	}
	if message, exceeded := allocatedToOthers.Add(increased).Exceeds(limitOfIncreased); exceeded {
		return fmt.Errorf("%w: %s", errExceeded, message)
	}
	return nil
}

// checkNotBelow checks that a parent quota (limit) is not lower than what its children are
// allocated, on every resource it limits. children names them in the error.
func checkNotBelow(limit, allocated spxQuota.Quantities, children string) error {
	if message, exceeded := allocated.Exceeds(limit); exceeded {
		return fmt.Errorf("%w by %s: %s", ErrBelowAllocated, children, message)
	}
	return nil
}

// loweredLimits compares the quota enforced in an AZ before and after a change, and returns the
// resources that were already limited before and are lowered, with their new value. A resource
// removed from the AZ quota but limited by a parent is lowered to 0.
func loweredLimits(before, after model.QuotaResources) (spxQuota.Quantities, error) {
	parsedBefore, err := before.Parse()
	if err != nil {
		return nil, err
	}
	parsedAfter, err := after.Parse()
	if err != nil {
		return nil, err
	}
	lowered := spxQuota.Quantities{}
	for name, newValue := range parsedAfter {
		if oldValue, limitedBefore := parsedBefore[name]; limitedBefore && newValue < oldValue {
			lowered[name] = newValue
		}
	}
	return lowered, nil
}

// checkUsage checks, with no transaction open, that the lowered resources of a project in an AZ
// stay at or above what the AZ reports the project uses: ErrBelowUsage, or ErrUsageUnavailable
// when the AZ cannot report it. A resource the AZ does not track yet has no usage to keep.
func checkUsage(ctx context.Context, organizationId, projectId uuid.UUID, codeAZ string, lowered spxQuota.Quantities) error {
	if len(lowered) == 0 {
		return nil
	}
	if ReadUsage == nil {
		return errors.New("no quota usage reader configured")
	}
	used, err := ReadUsage(ctx, organizationId, projectId, codeAZ)
	if err != nil {
		log := logger.GetLogger(ctx)
		log.Err(err).Str("az", codeAZ).Str("projectId", projectId.String()).Msg("failed to read quota usage")
		return fmt.Errorf("%w: %s", ErrUsageUnavailable, codeAZ)
	}
	for _, name := range slices.Sorted(maps.Keys(lowered)) {
		if value, tracked := used[name]; tracked && value > lowered[name] {
			return fmt.Errorf("%w: %s %s used in %s, above the requested %s", ErrBelowUsage,
				name, formatQuantity(name, value), codeAZ, formatQuantity(name, lowered[name]))
		}
	}
	return nil
}

// formatQuantity formats the quantity of one resource as in a quota (e.g. "500m", "1Gi").
func formatQuantity(name string, value int64) string {
	return spxQuota.Quantities{name: value}.Resources()[name]
}

// ---------------------------------------------------------------------------------------------
// Transactions and locks
// ---------------------------------------------------------------------------------------------

// inOrganizationTransaction runs fn in a transaction holding the organization lock (see
// lockOrganization).
func inOrganizationTransaction(ctx context.Context, organizationId uuid.UUID, fn func(transaction *gorm.DB) error) error {
	return db.Client.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := lockOrganization(transaction, organizationId); err != nil {
			return err
		}
		return fn(transaction)
	})
}

// inSnapshot runs fn in a read-only transaction reading a single snapshot of the database.
func inSnapshot(ctx context.Context, fn func(transaction *gorm.DB) error) error {
	return db.Client.WithContext(ctx).Transaction(fn, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
}

// inQuotaWrite runs fn in inOrganizationTransaction with a new quota revision of the organization.
func inQuotaWrite(ctx context.Context, organizationId uuid.UUID, fn func(transaction *gorm.DB, revision int64) error) error {
	return inOrganizationTransaction(ctx, organizationId, func(transaction *gorm.DB) error {
		revision, err := bumpRevision(transaction, organizationId)
		if err != nil {
			return err
		}
		return fn(transaction, revision)
	})
}

// inProjectTransaction runs fn in inQuotaWrite after checking that the project belongs to the
// organization (ErrProjectNotFound otherwise).
func inProjectTransaction(ctx context.Context, organizationId, projectId uuid.UUID, fn func(transaction *gorm.DB, revision int64) error) error {
	return inQuotaWrite(ctx, organizationId, func(transaction *gorm.DB, revision int64) error {
		if err := checkProject(transaction, organizationId, projectId); err != nil {
			return err
		}
		return fn(transaction, revision)
	})
}

// lockOrganization takes a row lock on the organization, serializing the quota writes of an
// organization so that concurrent requests cannot both pass the same capacity check.
func lockOrganization(transaction *gorm.DB, organizationId uuid.UUID) error {
	err := transaction.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Select("id").Where("id = ?", organizationId).Take(&model.Organization{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrOrganizationNotFound
	}
	return err
}

// bumpRevision gives the organization a new quota revision and returns it: the database clock in
// microseconds, or the previous revision plus one when the clock lags behind it.
func bumpRevision(transaction *gorm.DB, organizationId uuid.UUID) (int64, error) {
	var revision int64
	err := transaction.Raw(sqlBumpRevision, organizationId).Scan(&revision).Error
	return revision, err
}

// sqlBumpRevision gives an organization a new quota revision.
const sqlBumpRevision = "UPDATE organizations SET quota_revision = GREATEST(quota_revision + 1, " +
	"(extract(epoch FROM clock_timestamp()) * 1000000)::bigint) WHERE id = ? RETURNING quota_revision"

// checkProject ensures the project exists and belongs to the organization.
func checkProject(database *gorm.DB, organizationId, projectId uuid.UUID) error {
	err := database.Select("id").Where("id = ? AND orga_id = ?", projectId, organizationId).Take(&model.Project{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrProjectNotFound
	}
	return err
}

// ---------------------------------------------------------------------------------------------
// Database access
// ---------------------------------------------------------------------------------------------

// findOrganizationQuota returns the organization quota, or nil when it has none.
func findOrganizationQuota(database *gorm.DB, organizationId uuid.UUID) (*model.OrganizationQuota, error) {
	var organizationQuota model.OrganizationQuota
	err := database.Where("organization_id = ?", organizationId).Take(&organizationQuota).Error
	return nilWhenNotFound(&organizationQuota, err)
}

// findProjectQuota returns the project quota, or nil when it has none.
func findProjectQuota(database *gorm.DB, projectId uuid.UUID) (*model.ProjectQuota, error) {
	var projectQuota model.ProjectQuota
	err := database.Where("project_id = ?", projectId).Take(&projectQuota).Error
	return nilWhenNotFound(&projectQuota, err)
}

// nilWhenNotFound returns row, or nil (and no error) when the query found no row.
func nilWhenNotFound[T any](row *T, err error) (*T, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

// listAZQuotas lists the AZ quotas of a project, ordered by AZ.
func listAZQuotas(database *gorm.DB, projectId uuid.UUID) ([]model.ProjectAZQuota, error) {
	var azQuotas []model.ProjectAZQuota
	err := database.Where("project_id = ?", projectId).Order("code_az").Find(&azQuotas).Error
	return azQuotas, err
}

// listStates lists the sync states of a project, ordered by AZ.
func listStates(database *gorm.DB, projectId uuid.UUID) ([]model.ProjectAZQuotaState, error) {
	var states []model.ProjectAZQuotaState
	err := database.Where("project_id = ?", projectId).Order("code_az").Find(&states).Error
	return states, err
}

// listOrganizationStates lists the sync states of the live projects of the organization.
func listOrganizationStates(database *gorm.DB, organizationId uuid.UUID) ([]model.ProjectAZQuotaState, error) {
	var states []model.ProjectAZQuotaState
	err := database.Where("project_id IN (?)", liveProjectIds(database, organizationId)).
		Order("project_id, code_az").Find(&states).Error
	return states, err
}

// listOrganizationAZQuotas lists the AZ quotas of the live projects of the organization.
func listOrganizationAZQuotas(database *gorm.DB, organizationId uuid.UUID) ([]model.ProjectAZQuota, error) {
	var azQuotas []model.ProjectAZQuota
	err := database.Where("project_id IN (?)", liveProjectIds(database, organizationId)).
		Order("project_id, code_az").Find(&azQuotas).Error
	return azQuotas, err
}

// listProjectQuotas lists the quotas of the live projects of the organization. Quotas of
// deleted projects are ignored, so they do not keep consuming the organization quota.
func listProjectQuotas(database *gorm.DB, organizationId uuid.UUID) ([]model.ProjectQuota, error) {
	var projectQuotas []model.ProjectQuota
	err := database.Where("organization_id = ? AND project_id IN (?)", organizationId, liveProjectIds(database, organizationId)).
		Order("project_id").Find(&projectQuotas).Error
	return projectQuotas, err
}

// listProjectIds returns the IDs of the live projects of the organization, ordered.
func listProjectIds(database *gorm.DB, organizationId uuid.UUID) ([]uuid.UUID, error) {
	var projectIds []uuid.UUID
	err := database.Model(&model.Project{}).Where("orga_id = ?", organizationId).Order("id").Pluck("id", &projectIds).Error
	return projectIds, err
}

// liveProjectIds is the subquery of the IDs of the live projects of the organization.
func liveProjectIds(database *gorm.DB, organizationId uuid.UUID) *gorm.DB {
	return database.Session(&gorm.Session{NewDB: true}).Model(&model.Project{}).Select("id").Where("orga_id = ?", organizationId)
}

// upsertProjectQuota creates or replaces the quota of a project.
func upsertProjectQuota(transaction *gorm.DB, organizationId, projectId uuid.UUID, resources model.QuotaResources) (model.ProjectQuota, error) {
	projectQuota := model.ProjectQuota{ProjectId: projectId, OrganizationId: organizationId, Resources: resources}
	err := upsert(transaction, &projectQuota, []string{"project_id"}, "organization_id", "resources")
	return projectQuota, err
}

// upsertAZQuota creates or replaces the quota of a project in one AZ.
func upsertAZQuota(transaction *gorm.DB, projectId uuid.UUID, codeAZ string, resources model.QuotaResources) (model.ProjectAZQuota, error) {
	azQuota := model.ProjectAZQuota{ProjectId: projectId, CodeAZ: codeAZ, Resources: resources}
	err := upsert(transaction, &azQuota, []string{"project_id", "code_az"}, "resources")
	return azQuota, err
}

// markDirty marks the quota of the projects due in the given AZs at revision and resets the
// attempts and error of the previous push.
func markDirty(transaction *gorm.DB, organizationId uuid.UUID, revision int64, projectIds []uuid.UUID, codes []string) error {
	if len(projectIds) == 0 || len(codes) == 0 {
		return nil
	}
	states := make([]model.ProjectAZQuotaState, 0, len(projectIds)*len(codes))
	for _, projectId := range projectIds {
		for _, code := range codes {
			states = append(states, model.ProjectAZQuotaState{
				ProjectId: projectId, CodeAZ: code, OrganizationId: organizationId, WantedRevision: revision,
			})
		}
	}
	return transaction.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "project_id"}, {Name: "code_az"}},
		DoUpdates: clause.Assignments(map[string]any{
			"wanted_revision": gorm.Expr("excluded.wanted_revision"),
			"due_at":          gorm.Expr("now()"),
			"attempts":        gorm.Expr("0"),
			"last_error":      gorm.Expr("NULL"),
		}),
	}).Create(&states).Error
}

// markOrganizationDirty marks every live project of the organization dirty in all its AZs.
func markOrganizationDirty(transaction *gorm.DB, organizationId uuid.UUID, revision int64) error {
	codes := azCodes(organizationId)
	if len(codes) == 0 {
		return nil
	}
	projectIds, err := listProjectIds(transaction, organizationId)
	if err != nil {
		return err
	}
	return markDirty(transaction, organizationId, revision, projectIds, codes)
}

// azCodes returns the codes of the AZs of the organization, sorted.
func azCodes(organizationId uuid.UUID) []string {
	azConfigs := az.FindAll(organizationId.String())
	codes := make([]string, 0, len(azConfigs))
	for _, azConfig := range azConfigs {
		codes = append(codes, azConfig.Code)
	}
	slices.Sort(codes)
	return codes
}

// upsert inserts row, or updates the given columns when a row with the same conflict columns
// exists. A soft-deleted row is revived.
func upsert(transaction *gorm.DB, row any, conflictColumns []string, updatedColumns ...string) error {
	columns := make([]clause.Column, len(conflictColumns))
	for index, name := range conflictColumns {
		columns[index] = clause.Column{Name: name}
	}
	return transaction.Clauses(clause.OnConflict{
		Columns:   columns,
		DoUpdates: clause.AssignmentColumns(append(updatedColumns, "updated_at", "deleted_at")),
	}).Create(row).Error
}
