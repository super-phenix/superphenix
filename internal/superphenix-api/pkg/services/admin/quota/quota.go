package quota

import (
	"net/http"

	crudQuota "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/quotasync"
	quotaService "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/quota"
)

// GetOrganizationQuota handles GET /organization/{orgaId}
func (h *Service) GetOrganizationQuota(w http.ResponseWriter, r *http.Request) {
	orgID, ok := quotaService.ParseID(w, r, "orgaId")
	if !ok {
		return
	}
	overview, err := crudQuota.GetOrganizationQuotaOverview(r.Context(), orgID)
	if err != nil {
		quotaService.WriteError(w, r, err, "failed to get organization quota")
		return
	}
	quotaService.WriteJSON(w, http.StatusOK, overview)
}

// SetOrganizationQuota handles PUT /organization/{orgaId}
func (h *Service) SetOrganizationQuota(w http.ResponseWriter, r *http.Request) {
	orgID, ok := quotaService.ParseID(w, r, "orgaId")
	if !ok {
		return
	}
	req, ok := quotaService.DecodeResources(w, r)
	if !ok {
		return
	}
	oq, err := crudQuota.SetOrganizationQuota(r.Context(), orgID, req)
	if err != nil {
		quotaService.WriteError(w, r, err, "failed to set organization quota")
		return
	}
	if err := quotasync.Organization(r.Context(), orgID); err != nil {
		quotaService.WriteError(w, r, err, "failed to apply organization quota")
		return
	}
	quotaService.WriteJSON(w, http.StatusOK, oq)
}

// DeleteOrganizationQuota handles DELETE /organization/{orgaId}
func (h *Service) DeleteOrganizationQuota(w http.ResponseWriter, r *http.Request) {
	orgID, ok := quotaService.ParseID(w, r, "orgaId")
	if !ok {
		return
	}
	if err := crudQuota.DeleteOrganizationQuota(r.Context(), orgID); err != nil {
		quotaService.WriteError(w, r, err, "failed to delete organization quota")
		return
	}
	if err := quotasync.Organization(r.Context(), orgID); err != nil {
		quotaService.WriteError(w, r, err, "failed to apply organization quota")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
