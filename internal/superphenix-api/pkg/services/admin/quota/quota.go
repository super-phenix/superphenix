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

type AdminOrganizationQuotaResponse struct {
	OrganizationId string                `json:"organizationId"`
	Unlimited      bool                  `json:"unlimited"`
	Resources      *model.QuotaResources `json:"resources,omitempty"`
	Allocated      model.QuotaResources  `json:"allocated"`
	Remaining      *model.QuotaResources `json:"remaining,omitempty"`
	ProjectQuotas  []model.ProjectQuota  `json:"projectQuotas"`
}

// GetOrganizationQuota handles GET /organization/{orgaId}
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

	resp := AdminOrganizationQuotaResponse{
		OrganizationId: orgID.String(),
		Allocated:      totalAllocated.ToQuotaResources(),
		ProjectQuotas:  pqs,
	}

	if oq == nil {
		resp.Unlimited = true
	} else {
		resp.Unlimited = false
		resp.Resources = &oq.Resources
		parsedOrg, _ := crudQuota.ParseResources(oq.Resources)
		remaining := parsedOrg.Sub(totalAllocated)
		remRes := remaining.ToQuotaResources()
		resp.Remaining = &remRes
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SetOrganizationQuota handles PUT /organization/{orgaId}
func (h *Service) SetOrganizationQuota(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaIdStr := chi.URLParam(r, "orgaId")
	orgID, err := uuid.Parse(orgaIdStr)
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("invalid organization id")
		return
	}

	var req model.QuotaResources
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("invalid request body")
		return
	}

	oq, err := crudQuota.SetOrganizationQuota(r.Context(), orgID, req)
	if err != nil {
		if err == crudQuota.ErrOrgQuotaExceeded || err == crudQuota.ErrInvalidQuantity {
			httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
			return
		}
		log.Err(err).Str("orgaId", orgaIdStr).Msg("failed to set organization quota")
		httpError.Http(w, r, http.StatusInternalServerError).Msg(err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(oq)
}

// DeleteOrganizationQuota handles DELETE /organization/{orgaId}
func (h *Service) DeleteOrganizationQuota(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaIdStr := chi.URLParam(r, "orgaId")
	orgID, err := uuid.Parse(orgaIdStr)
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("invalid organization id")
		return
	}

	if err := crudQuota.DeleteOrganizationQuota(r.Context(), orgID); err != nil {
		log.Err(err).Str("orgaId", orgaIdStr).Msg("failed to delete organization quota")
		httpError.Http(w, r, http.StatusInternalServerError).Msg(err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
