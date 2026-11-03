package quota

// Sync states: the quota of a project in one AZ is pushed to the AZ controller by the quotasync
// reconciler, which may run on several API replicas at once. Each push goes through:
//   - Claim or ClaimDirty: the state is leased to one reconciler (a lease that elapses lets
//     another one take it over);
//   - Prepare: under the organization lock, the push gets a new quota revision, which becomes the
//     prepared revision of the state, and its AZ quota is added to the held quota (see
//     azAllocation), all before anything is sent;
//   - the push itself, without any transaction open;
//   - Record: the outcome is recorded only if no other push of the state was prepared since.
//
// The AZ controller refuses a push older than the revision it holds, so an older push never
// overwrites a newer one in the cluster, and Record never lets an older push overwrite a newer
// outcome in the database.

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/az"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
)

// StateKey identifies the sync state of the quota of a project in one AZ.
type StateKey struct {
	OrganizationId uuid.UUID
	ProjectId      uuid.UUID
	CodeAZ         string
}

// Push is the push of the quota of a project to one AZ, prepared by Prepare.
type Push struct {
	StateKey
	Revision int64
	// Hard is the Kubernetes quota to push (the ResourceQuota `spec.hard`).
	Hard map[string]string
	// AZQuota is the AZ quota the push enforces: the held quota once the AZ applies it.
	AZQuota model.QuotaResources
}

// maxErrorLength bounds the error recorded for a failed push.
const maxErrorLength = 1024

// Claim leases, to the reconciler identified by token and for lease, up to limit states of the
// given AZs that are due for a push. The states other reconcilers are claiming at the same time
// are skipped.
func Claim(ctx context.Context, token uuid.UUID, codes []string, limit int, lease time.Duration) ([]StateKey, error) {
	if len(codes) == 0 || limit <= 0 {
		return nil, nil
	}
	due := db.Client.Model(&model.ProjectAZQuotaState{}).Select("project_id", "code_az").
		Where("due_at <= now() AND code_az IN ?", codes).
		Order("due_at").
		Limit(limit).
		Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate, Options: clause.LockingOptionsSkipLocked})
	return leaseStates(ctx, due, token, lease)
}

// ClaimDirty leases, to the reconciler identified by token and for lease, the states of the
// organization in its AZs (only those of projectId when not nil, and of codeAZ when not empty)
// whose last change is not applied yet, even those another reconciler holds: the push it prepares
// next then supersedes theirs (see Record). The states of an AZ the organization no longer has are
// left to the reconciler.
func ClaimDirty(ctx context.Context, token, organizationId uuid.UUID, projectId *uuid.UUID, codeAZ string, lease time.Duration) ([]StateKey, error) {
	codes := azCodes(organizationId)
	if codeAZ != "" {
		codes = slices.DeleteFunc(codes, func(code string) bool { return code != codeAZ })
	}
	if len(codes) == 0 {
		return nil, nil
	}
	due := db.Client.Model(&model.ProjectAZQuotaState{}).Select("project_id", "code_az").
		Where("organization_id = ? AND code_az IN ? AND wanted_revision > applied_revision", organizationId, codes).
		Order(`project_id, code_az COLLATE "C"`). // the order of sortedKeys
		Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate})
	if projectId != nil {
		due = due.Where("project_id = ?", *projectId)
	}
	return leaseStates(ctx, due, token, lease)
}

// leaseStates leases the states selected by due to the reconciler identified by token, for lease.
func leaseStates(ctx context.Context, due *gorm.DB, token uuid.UUID, lease time.Duration) ([]StateKey, error) {
	var keys []StateKey
	err := db.Client.WithContext(ctx).Raw(`
		WITH due AS (?)
		UPDATE project_az_quota_states AS s SET lease_token = ?, due_at = now() + make_interval(secs => ?)
		FROM due WHERE s.project_id = due.project_id AND s.code_az = due.code_az
		RETURNING s.organization_id, s.project_id, s.code_az`,
		due, token, lease.Seconds()).Scan(&keys).Error
	return keys, err
}

// Prepare prepares the push of states of the organization claimed by the reconciler identified by
// token, in one transaction holding the organization lock so that no quota write runs in between:
// the organization gets a new quota revision, which becomes the prepared revision of every state,
// and the AZ quota to push is added to the held quota of the state. A state claimed since by
// another reconciler is skipped. The states of a deleted organization or project are deleted, and
// a state of an AZ the organization no longer has is postponed by retryAfter with an error.
func Prepare(ctx context.Context, token, organizationId uuid.UUID, keys []StateKey, retryAfter time.Duration) ([]Push, error) {
	var pushes []Push
	err := db.Client.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		pushes = nil
		err := lockOrganization(transaction, organizationId)
		if errors.Is(err, ErrOrganizationNotFound) {
			return transaction.Where("organization_id = ?", organizationId).Delete(&model.ProjectAZQuotaState{}).Error
		}
		if err != nil {
			return err
		}
		revision, err := bumpRevision(transaction, organizationId)
		if err != nil {
			return err
		}
		hierarchies, err := readOrganizationHierarchies(transaction, organizationId)
		if err != nil {
			return err
		}
		for _, key := range sortedKeys(keys) {
			push, err := prepareState(transaction, token, key, hierarchies, revision, retryAfter)
			if err != nil {
				return err
			}
			if push != nil {
				pushes = append(pushes, *push)
			}
		}
		return nil
	})
	return pushes, err
}

// sortedKeys returns keys sorted by project then AZ, byte-wise like `ORDER BY project_id, code_az
// COLLATE "C"`: the order every transaction locks sync states in, so that none waits for another
// that waits for it.
func sortedKeys(keys []StateKey) []StateKey {
	sorted := slices.Clone(keys)
	slices.SortFunc(sorted, func(first, second StateKey) int {
		if order := slices.Compare(first.ProjectId[:], second.ProjectId[:]); order != 0 {
			return order
		}
		return strings.Compare(first.CodeAZ, second.CodeAZ)
	})
	return sorted
}

// prepareState prepares the push of one state at revision (see Prepare). It returns nil when it
// has nothing to push.
func prepareState(transaction *gorm.DB, token uuid.UUID, key StateKey, hierarchies map[uuid.UUID]Hierarchy,
	revision int64, retryAfter time.Duration) (*Push, error) {
	hierarchy, live := hierarchies[key.ProjectId]
	if !live {
		return nil, transaction.Where("project_id = ?", key.ProjectId).Delete(&model.ProjectAZQuotaState{}).Error
	}
	var state model.ProjectAZQuotaState
	err := transaction.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where("project_id = ? AND code_az = ? AND lease_token = ?", key.ProjectId, key.CodeAZ, token).Take(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	row := transaction.Model(&model.ProjectAZQuotaState{}).Where("project_id = ? AND code_az = ?", key.ProjectId, key.CodeAZ)
	if _, err := az.GetByCode(key.CodeAZ, key.OrganizationId.String()); err != nil {
		return nil, row.Updates(map[string]any{
			"last_error":  "availability zone not available to the organization",
			"lease_token": nil,
			"due_at":      gorm.Expr("now() + make_interval(secs => ?)", retryAfter.Seconds()),
		}).Error
	}

	azQuota := hierarchy.AZs[key.CodeAZ]
	held, err := azAllocation(azQuota, state.Held)
	if err != nil {
		return nil, err
	}
	encodedHeld, err := json.Marshal(held.Resources())
	if err != nil {
		return nil, err
	}
	err = row.Updates(map[string]any{"held": string(encodedHeld), "prepared_revision": revision}).Error
	if err != nil {
		return nil, err
	}
	return &Push{
		StateKey: key,
		Revision: revision,
		Hard:     hierarchy.AZ(key.CodeAZ).ToKubernetesQuota(),
		AZQuota:  azQuota,
	}, nil
}

// Record records the outcome of a push (pushErr nil when the AZ applied it), unless another push
// of the state was prepared since. An applied push makes its AZ quota the held quota, and the
// state is due again after resync. A failed push keeps the held quota, since the AZ may have
// applied it, and the state is due again after an exponential backoff starting at backoff and
// capped at maxBackoff.
func Record(ctx context.Context, push Push, pushErr error, resync, backoff, maxBackoff time.Duration) error {
	var updates map[string]any
	if pushErr != nil {
		message := pushErr.Error()
		if len(message) > maxErrorLength {
			message = message[:maxErrorLength]
		}
		delay := gorm.Expr("LEAST(? * power(2, LEAST(attempts, 20)), ?)", backoff.Seconds(), maxBackoff.Seconds())
		updates = map[string]any{
			"attempts":   gorm.Expr("attempts + 1"),
			"last_error": message,
			"due_at":     dueAt(push.Revision, delay),
		}
	} else {
		azQuota := push.AZQuota
		if azQuota == nil {
			azQuota = model.QuotaResources{}
		}
		held, err := json.Marshal(azQuota)
		if err != nil {
			return err
		}
		updates = map[string]any{
			"held":             string(held),
			"applied_revision": push.Revision,
			"attempts":         0,
			"last_error":       nil,
			"due_at":           dueAt(push.Revision, resync.Seconds()),
		}
	}
	updates["lease_token"] = nil
	return db.Client.WithContext(ctx).Session(&gorm.Session{SkipDefaultTransaction: true}).
		Model(&model.ProjectAZQuotaState{}).
		Where("project_id = ? AND code_az = ? AND prepared_revision = ?", push.ProjectId, push.CodeAZ, push.Revision).
		Updates(updates).Error
}

// dueAt is when a state recorded at revision is due next: at once when a write changed it since,
// otherwise after delay seconds.
func dueAt(revision int64, delay any) clause.Expr {
	return gorm.Expr("CASE WHEN wanted_revision > ? THEN now() ELSE now() + make_interval(secs => ?) END", revision, delay)
}

// EnsureStates creates the missing sync states of the live projects in the AZs of their
// organization, due at once, so that their quota gets pushed (e.g. an AZ added to the
// configuration). A state created for a project deleted meanwhile is deleted by Prepare.
func EnsureStates(ctx context.Context) error {
	projects, err := ListAllProjects(ctx)
	if err != nil {
		return err
	}
	var states []model.ProjectAZQuotaState
	for _, project := range projects {
		for _, code := range azCodes(project.OrgaId) {
			states = append(states, model.ProjectAZQuotaState{
				ProjectId: project.ID, CodeAZ: code, OrganizationId: project.OrgaId, WantedRevision: 1,
			})
		}
	}
	if len(states) == 0 {
		return nil
	}
	// Each batch commits on its own, so that no inserted state stays locked until the last one.
	return db.Client.WithContext(ctx).Session(&gorm.Session{SkipDefaultTransaction: true}).
		Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&states, 500).Error
}

// MarkNewProject asks the quota reconciler to push the quota of a project just created to every AZ
// of the organization. When an AZ has limits to enforce, it also deactivates the project
// (model.DeactivationQuotaFailedToApply) in the same transaction, until those AZs apply the quota
// (see quotasync.Initialize).
func MarkNewProject(ctx context.Context, organizationId, projectId uuid.UUID) error {
	return inProjectTransaction(ctx, organizationId, projectId, func(transaction *gorm.DB, revision int64) error {
		hierarchy, err := loadHierarchy(transaction, organizationId, projectId)
		if err != nil {
			return err
		}
		codes := azCodes(organizationId)
		unapplied := hierarchy.unappliedAZs(codes)
		if len(unapplied) > 0 {
			deactivation := model.ProjectDeactivation{
				ProjectId: projectId,
				Reason:    model.DeactivationQuotaFailedToApply,
				Message:   "quota not applied yet on: " + strings.Join(unapplied, ", "),
			}
			err := transaction.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "project_id"}, {Name: "reason"}},
				DoUpdates: clause.AssignmentColumns([]string{"message", "updated_at"}),
			}).Create(&deactivation).Error
			if err != nil {
				return err
			}
		}
		return markDirty(transaction, organizationId, revision, []uuid.UUID{projectId}, codes)
	})
}

// UnappliedAZs returns, sorted, the AZs of the organization that have something to enforce for the
// project but did not apply its quota yet.
func UnappliedAZs(ctx context.Context, organizationId, projectId uuid.UUID) ([]string, error) {
	hierarchy, err := GetHierarchy(ctx, organizationId, projectId)
	if err != nil {
		return nil, err
	}
	return hierarchy.unappliedAZs(azCodes(organizationId)), nil
}

// unappliedAZs returns the AZs of codes that have something to enforce for the project but did
// not apply its quota yet.
func (hierarchy Hierarchy) unappliedAZs(codes []string) []string {
	var unapplied []string
	for _, code := range codes {
		if len(hierarchy.AZ(code)) == 0 {
			continue
		}
		if status, _ := Status(hierarchy.States[code]); status != StatusApplied {
			unapplied = append(unapplied, code)
		}
	}
	return unapplied
}

// readOrganizationHierarchies reads the Hierarchy of every live project of the organization.
func readOrganizationHierarchies(transaction *gorm.DB, organizationId uuid.UUID) (map[uuid.UUID]Hierarchy, error) {
	organizationQuota, err := findOrganizationQuota(transaction, organizationId)
	if err != nil {
		return nil, err
	}
	projectIds, err := listProjectIds(transaction, organizationId)
	if err != nil {
		return nil, err
	}
	projectQuotas, err := listProjectQuotas(transaction, organizationId)
	if err != nil {
		return nil, err
	}
	azQuotas, err := listOrganizationAZQuotas(transaction, organizationId)
	if err != nil {
		return nil, err
	}
	states, err := listOrganizationStates(transaction, organizationId)
	if err != nil {
		return nil, err
	}

	hierarchies := make(map[uuid.UUID]Hierarchy, len(projectIds))
	for _, projectId := range projectIds {
		hierarchy := newHierarchy()
		if organizationQuota != nil {
			hierarchy.Organization = organizationQuota.Resources
		}
		hierarchies[projectId] = hierarchy
	}
	for _, projectQuota := range projectQuotas {
		if hierarchy, live := hierarchies[projectQuota.ProjectId]; live {
			hierarchy.Project = projectQuota.Resources
			hierarchies[projectQuota.ProjectId] = hierarchy
		}
	}
	for _, azQuota := range azQuotas {
		if hierarchy, live := hierarchies[azQuota.ProjectId]; live {
			hierarchy.AZs[azQuota.CodeAZ] = azQuota.Resources
		}
	}
	for _, state := range states {
		if hierarchy, live := hierarchies[state.ProjectId]; live {
			hierarchy.States[state.CodeAZ] = state
		}
	}
	return hierarchies, nil
}

// GroupByOrganization groups keys by organization, keeping their order; the organizations are
// sorted.
func GroupByOrganization(keys []StateKey) ([]uuid.UUID, map[uuid.UUID][]StateKey) {
	groups := map[uuid.UUID][]StateKey{}
	var organizationIds []uuid.UUID
	for _, key := range keys {
		if _, seen := groups[key.OrganizationId]; !seen {
			organizationIds = append(organizationIds, key.OrganizationId)
		}
		groups[key.OrganizationId] = append(groups[key.OrganizationId], key)
	}
	slices.SortFunc(organizationIds, func(first, second uuid.UUID) int {
		return slices.Compare(first[:], second[:])
	})
	return organizationIds, groups
}
