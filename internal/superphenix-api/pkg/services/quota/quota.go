package quota

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	crudQuota "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

type OrganizationQuotaOverview struct {
	OrganizationId string                 `json:"organizationId"`
	Unlimited      bool                   `json:"unlimited"`
	Resources      *model.QuotaResources  `json:"resources,omitempty"`
	Allocated      model.QuotaResources   `json:"allocated"`
	Remaining      *model.QuotaResources  `json:"remaining,omitempty"`
	ProjectQuotas  []model.ProjectQuota   `json:"projectQuotas"`
}

type ProjectQuotaOverview struct {
	ProjectId      string                 `json:"projectId"`
	OrganizationId string                 `json:"organizationId"`
	Resources      *model.QuotaResources  `json:"resources,omitempty"`
	AZQuotas       []model.ProjectAZQuota `json:"azQuotas"`
}

// GetOrganizationQuota handles GET /organization/{orgaId}/quota
func (h *Service) GetOrganizationQuota(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaIdStr := chi.URLParam(r, "orgaId")
	orgID, err := uuid.Parse(orgaIdStr)
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("invalid organization id")
		return
	}

	oq, err := crudQuota.GetOrganizationQuota(r.Context(), orgID)
	if err != nil {
		log.Err(err).Str("orgaId", orgaIdStr).Msg("failed to get organization quota")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to get organization quota")
		return
	}

	pqs, err := crudQuota.ListProjectQuotasByOrg(r.Context(), orgID)
	if err != nil {
		log.Err(err).Str("orgaId", orgaIdStr).Msg("failed to list project quotas")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to list project quotas")
		return
	}

	var totalAllocated crudQuota.ParsedResources
	for _, pq := range pqs {
		parsed, _ := crudQuota.ParseResources(pq.Resources)
		totalAllocated = totalAllocated.Add(parsed)
	}

	overview := OrganizationQuotaOverview{
		OrganizationId: orgID.String(),
		Allocated:      totalAllocated.ToQuotaResources(),
		ProjectQuotas:  pqs,
	}

	if oq == nil {
		overview.Unlimited = true
	} else {
		overview.Unlimited = false
		overview.Resources = &oq.Resources
		parsedOrg, _ := crudQuota.ParseResources(oq.Resources)
		remaining := parsedOrg.Sub(totalAllocated)
		remRes := remaining.ToQuotaResources()
		overview.Remaining = &remRes
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(overview)
}

// GetProjectQuota handles GET /organization/{orgaId}/project/{projectId}/quota
func (h *Service) GetProjectQuota(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaIdStr := chi.URLParam(r, "orgaId")
	projIdStr := chi.URLParam(r, "projectId")

	projID, err := uuid.Parse(projIdStr)
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("invalid project id")
		return
	}

	pq, err := crudQuota.GetProjectQuota(r.Context(), projID)
	if err != nil {
		log.Err(err).Str("projectId", projIdStr).Msg("failed to get project quota")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to get project quota")
		return
	}

	azqs, err := crudQuota.ListProjectAZQuotas(r.Context(), projID)
	if err != nil {
		log.Err(err).Str("projectId", projIdStr).Msg("failed to list project az quotas")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to list project az quotas")
		return
	}

	overview := ProjectQuotaOverview{
		ProjectId:      projID.String(),
		OrganizationId: orgaIdStr,
		AZQuotas:       azqs,
	}
	if pq != nil {
		overview.Resources = &pq.Resources
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(overview)
}

// SetProjectQuota handles PUT /organization/{orgaId}/project/{projectId}/quota
func (h *Service) SetProjectQuota(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaIdStr := chi.URLParam(r, "orgaId")
	projIdStr := chi.URLParam(r, "projectId")

	orgID, err := uuid.Parse(orgaIdStr)
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("invalid organization id")
		return
	}
	projID, err := uuid.Parse(projIdStr)
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("invalid project id")
		return
	}

	var req model.QuotaResources
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("invalid request body")
		return
	}

	pq, err := crudQuota.SetProjectQuota(r.Context(), orgID, projID, req)
	if err != nil {
		if err == crudQuota.ErrOrgQuotaExceeded || err == crudQuota.ErrInvalidQuantity {
			httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
			return
		}
		log.Err(err).Str("projectId", projIdStr).Msg("failed to set project quota")
		httpError.Http(w, r, http.StatusInternalServerError).Msg(err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(pq)
}

// SetProjectAZQuota handles PUT /organization/{orgaId}/project/{projectId}/quota/az/{codeAZ}
func (h *Service) SetProjectAZQuota(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	projIdStr := chi.URLParam(r, "projectId")
	codeAZ := chi.URLParam(r, "codeAZ")

	projID, err := uuid.Parse(projIdStr)
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("invalid project id")
		return
	}
	if codeAZ == "" {
		httpError.Http(w, r, http.StatusBadRequest).Msg("codeAZ is required")
		return
	}

	var req model.QuotaResources
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("invalid request body")
		return
	}

	azq, err := crudQuota.SetProjectAZQuota(r.Context(), projID, codeAZ, req)
	if err != nil {
		if err == crudQuota.ErrProjectQuotaExceeded || err == crudQuota.ErrInvalidQuantity {
			httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
			return
		}
		log.Err(err).Str("projectId", projIdStr).Str("codeAZ", codeAZ).Msg("failed to set project az quota")
		httpError.Http(w, r, http.StatusInternalServerError).Msg(err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(azq)
}

// DeleteProjectQuota handles DELETE /organization/{orgaId}/project/{projectId}/quota
func (h *Service) DeleteProjectQuota(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	projIdStr := chi.URLParam(r, "projectId")

	projID, err := uuid.Parse(projIdStr)
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("invalid project id")
		return
	}

	if err := crudQuota.DeleteProjectQuota(r.Context(), projID); err != nil {
		log.Err(err).Str("projectId", projIdStr).Msg("failed to delete project quota")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to delete project quota")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
