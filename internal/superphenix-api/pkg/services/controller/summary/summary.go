package summary

import (
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authorization"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/product"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	ctrlutils "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/utils"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/entity"
	pw "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/permission"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const recentProductLimit = 5

// ProjectSummaryResponse is the resource overview of a project.
// Product types the caller may not read are absent from Counts, CountsByAZ and Recent.
type ProjectSummaryResponse struct {
	Counts     map[string]int64 `json:"counts"`
	CountsByAZ map[string]int64 `json:"countsByAz"`
	Quota      ProjectQuota     `json:"quota"`
	Recent     []RecentProduct  `json:"recent"`
}

// ProjectQuota is the product creation quota of a project. Used counts every product type,
// regardless of the caller's read permissions.
type ProjectQuota struct {
	Used  int64 `json:"used"`
	Limit int64 `json:"limit"`
}

// RecentProduct is one recently created product.
type RecentProduct struct {
	ID          string    `json:"id"`
	EffectiveID string    `json:"eid"`
	ProductName string    `json:"productName"`
	ProductType string    `json:"productType"`
	CodeAZ      string    `json:"codeAz"`
	CreatedAt   time.Time `json:"createdAt"`
}

// GetProjectSummary
//
//	@Summary		Retrieve a project resource overview
//	@Description	Report the project resources of each product type the caller is allowed to read:
//	@Description	counts, spread over the AZs, product creation quota usage and last created ones.
//	@Description	GitOps-managed resources are not reported.
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path		string					true	"Organization ID"
//	@Param			projectId	path		string					true	"Project ID"
//	@Success		200			{object}	ProjectSummaryResponse	"Resource counts per product type"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{projectId}/summary [get]
//	@Security		Bearer[OrganizationRead, ProjectRead]
func (h *Service) GetProjectSummary(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	_, project, code, errMsg := ctrlutils.CheckListPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	readableTypes, err := readableProductTypes(r)
	if err != nil {
		log.Err(err).Str("projectId", project.ID.String()).Msg(consts.SpxListPermissionsError)
		httpError.Http(w, r, consts.SpxListPermissionsErrorCode).Msg(consts.SpxListPermissionsError)
		return
	}

	summary, err := buildSummary(project.ID, readableTypes)
	if err != nil {
		log.Err(err).Str("projectId", project.ID.String()).Msg(consts.SpxFindAllResourcesError)
		httpError.Http(w, r, consts.SpxFindAllResourcesErrorCode).Msg(consts.SpxFindAllResourcesError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(summary)
	if err != nil {
		log.Err(err).Msg(consts.SpxResponseParseFailure)
		httpError.Http(w, r, consts.SpxResponseParseFailureCode).Msg(consts.SpxResponseParseFailure)
		return
	}
	_, _ = w.Write(marshal)
}

// readableProductTypes returns the product types the caller may read, from a single permission
// lookup. Super admins get every type.
func readableProductTypes(r *http.Request) ([]string, error) {
	userId, ok := r.Context().Value(consts.ContextUserId).(string)
	if !ok || userId == "" {
		return nil, errors.New("no user id found in context")
	}

	if authorization.IsSuperAdminUser(userId) {
		return slices.Sorted(maps.Keys(model.ProductTypeReadPermission)), nil
	}

	permissions, err := pw.ListUserPermission(
		r.Context(),
		chi.URLParam(r, "orgaId"),
		entity.Project,
		chi.URLParam(r, "projectId"),
		userId,
	)
	if err != nil {
		return nil, err
	}

	granted := make(map[string]bool, len(permissions))
	for _, permission := range permissions {
		granted[permission] = true
	}

	return filterReadable(granted), nil
}

// filterReadable returns, sorted, the product types whose read permission is granted.
func filterReadable(granted map[string]bool) []string {
	types := make([]string, 0, len(model.ProductTypeReadPermission))
	for productType, permission := range model.ProductTypeReadPermission {
		if granted[permission] {
			types = append(types, productType)
		}
	}
	slices.Sort(types)

	return types
}

func buildSummary(projectId uuid.UUID, readableTypes []string) (ProjectSummaryResponse, error) {
	counts, err := product.CountByTypeForProject(projectId, readableTypes)
	if err != nil {
		return ProjectSummaryResponse{}, err
	}

	countsByAZ, err := product.CountByAZForProject(projectId, readableTypes)
	if err != nil {
		return ProjectSummaryResponse{}, err
	}

	projectQuota, err := buildQuota(projectId)
	if err != nil {
		return ProjectSummaryResponse{}, err
	}

	recent, err := buildRecent(projectId, readableTypes)
	if err != nil {
		return ProjectSummaryResponse{}, err
	}

	return ProjectSummaryResponse{
		Counts:     counts,
		CountsByAZ: countsByAZ,
		Quota:      projectQuota,
		Recent:     recent,
	}, nil
}

func buildQuota(projectId uuid.UUID) (ProjectQuota, error) {
	used, err := product.CountForProject(projectId)
	if err != nil {
		return ProjectQuota{}, err
	}

	quotaEntity, err := quota.GetQuotaValue(
		quota.ProjectLimitProduct,
		projectId,
		quota.MapQuotaEntityType[quota.ProjectLimitProduct],
	)
	if err != nil {
		return ProjectQuota{}, err
	}

	limit, err := strconv.ParseInt(quotaEntity.Value, 10, 64)
	if err != nil {
		return ProjectQuota{}, err
	}

	return ProjectQuota{Used: used, Limit: limit}, nil
}

func buildRecent(projectId uuid.UUID, readableTypes []string) ([]RecentProduct, error) {
	products, err := product.FindLastCreatedByProject(projectId, readableTypes, recentProductLimit)
	if err != nil {
		return nil, err
	}

	recent := make([]RecentProduct, 0, len(products))
	for _, p := range products {
		recent = append(recent, RecentProduct{
			ID:          p.ID.String(),
			EffectiveID: p.EffectiveID,
			ProductName: p.ProductName,
			ProductType: p.ProductTypeId,
			CodeAZ:      p.CodeAZ,
			CreatedAt:   p.CreatedAt,
		})
	}

	return recent, nil
}
