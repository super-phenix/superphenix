package quota

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	crudQuota "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/quotasync"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

// maxBodySizeMB bounds the size of a quota request body.
const maxBodySizeMB = 1

// GetOrganizationQuota handles GET /organization/{orgaId}/quota
//
//	@Summary		Get the organization quota
//	@Description	Get the quota of an organization (unlimited when not defined), what is allocated to its projects and what remains
//	@Tags			v1, quota
//	@Produce		json
//	@Param			orgaId	path		string								true	"Organization ID"
//	@Success		200		{object}	crudQuota.OrganizationQuotaOverview	"Organization quota"
//	@Failure		400
//	@Failure		403
//	@Failure		404
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/quota [get]
//	@Security		Bearer[OrganizationRead, OrganizationProjectQuotaRead]
func (h *Service) GetOrganizationQuota(w http.ResponseWriter, r *http.Request) {
	orgID, ok := ParseID(w, r, "orgaId")
	if !ok {
		return
	}
	overview, err := crudQuota.GetOrganizationQuotaOverview(r.Context(), orgID)
	if err != nil {
		WriteError(w, r, err, "failed to get organization quota")
		return
	}
	WriteJSON(w, http.StatusOK, overview)
}

// GetProjectQuota handles GET /organization/{orgaId}/project/{projectId}/quota
//
//	@Summary		Get the project quota
//	@Description	Get the quota of a project (unlimited when not defined), what is allocated to its AZs and what remains, and for every AZ of the organization its AZ quota and what the project currently uses there (ResourceQuota status.used). An AZ whose usage cannot be read is returned with available=false.
//	@Tags			v1, quota
//	@Produce		json
//	@Param			orgaId		path		string							true	"Organization ID"
//	@Param			projectId	path		string							true	"Project ID"
//	@Success		200			{object}	crudQuota.ProjectQuotaOverview	"Project quota"
//	@Failure		400
//	@Failure		403
//	@Failure		404
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/project/{projectId}/quota [get]
//	@Security		Bearer[OrganizationRead, ProjectAZQuotaRead]
func (h *Service) GetProjectQuota(w http.ResponseWriter, r *http.Request) {
	orgID, projID, ok := parseOrgAndProject(w, r)
	if !ok {
		return
	}
	overview, err := quotasync.GetProjectQuotaOverview(r.Context(), orgID, projID)
	if err != nil {
		WriteError(w, r, err, "failed to get project quota")
		return
	}
	WriteJSON(w, http.StatusOK, overview)
}

// SetProjectQuota handles PUT /organization/{orgaId}/project/{projectId}/quota
//
//	@Summary		Set the project quota
//	@Description	Set (replace) the quota of a project and apply it to every AZ. Allowed resources: cpu, memory, disk. It must fit in the organization quota and cannot be lower than the sum of its AZ quotas. It is split equally among the AZs only when the project has no AZ quota yet.
//	@Tags			v1, quota
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path		string				true	"Organization ID"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			Body		body		map[string]string	true	"Quota, e.g. {\"cpu\": \"16\", \"memory\": \"64Gi\", \"disk\": \"1Ti\"}"
//	@Success		200			{object}	model.ProjectQuota	"Project quota"
//	@Failure		400			"Invalid quota, exceeds the organization quota or below the AZ quotas"
//	@Failure		403
//	@Failure		404
//	@Failure		413
//	@Failure		415
//	@Failure		500
//	@Failure		502	"Saved but not applied on some AZs (retry to apply it again)"
//	@Router			/v1/organization/{orgaId}/project/{projectId}/quota [put]
//	@Security		Bearer[OrganizationRead, OrganizationProjectQuotaWrite]
func (h *Service) SetProjectQuota(w http.ResponseWriter, r *http.Request) {
	orgID, projID, ok := parseOrgAndProject(w, r)
	if !ok {
		return
	}
	req, ok := DecodeResources(w, r)
	if !ok {
		return
	}
	pq, err := crudQuota.SetProjectQuota(r.Context(), orgID, projID, req)
	if err != nil {
		WriteError(w, r, err, "failed to set project quota")
		return
	}
	if err := quotasync.Project(r.Context(), orgID, projID); err != nil {
		WriteError(w, r, err, "failed to apply project quota")
		return
	}
	WriteJSON(w, http.StatusOK, pq)
}

// SetProjectAZQuota handles PUT /organization/{orgaId}/project/{projectId}/quota/az/{codeAZ}
//
//	@Summary		Set the project AZ quota
//	@Description	Set (replace) the quota of a project in one AZ and apply it. Allowed resources: cpu, memory, disk. It must fit in the project quota (or the organization quota when the project has none) and cannot be lowered below what the project currently uses in the AZ.
//	@Tags			v1, quota
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path		string					true	"Organization ID"
//	@Param			projectId	path		string					true	"Project ID"
//	@Param			codeAZ		path		string					true	"AZ code"
//	@Param			Body		body		map[string]string		true	"Quota, e.g. {\"cpu\": \"8\", \"memory\": \"32Gi\", \"disk\": \"500Gi\"}"
//	@Success		200			{object}	model.ProjectAZQuota	"Project AZ quota"
//	@Failure		400			"Invalid quota, exceeds the parent quota or below the current usage"
//	@Failure		403
//	@Failure		404
//	@Failure		413
//	@Failure		415
//	@Failure		500
//	@Failure		502	"Saved but not applied on the AZ (retry to apply it again)"
//	@Failure		503	"AZ or its usage unavailable, nothing changed"
//	@Router			/v1/organization/{orgaId}/project/{projectId}/quota/az/{codeAZ} [put]
//	@Security		Bearer[OrganizationRead, ProjectAZQuotaWrite]
func (h *Service) SetProjectAZQuota(w http.ResponseWriter, r *http.Request) {
	orgID, projID, ok := parseOrgAndProject(w, r)
	if !ok {
		return
	}
	req, ok := DecodeResources(w, r)
	if !ok {
		return
	}
	codeAZ := chi.URLParam(r, "codeAZ")
	azq, err := crudQuota.SetProjectAZQuota(r.Context(), orgID, projID, codeAZ, req)
	if err != nil {
		WriteError(w, r, err, "failed to set project az quota")
		return
	}
	if err := quotasync.Project(r.Context(), orgID, projID, codeAZ); err != nil {
		WriteError(w, r, err, "failed to apply project az quota")
		return
	}
	WriteJSON(w, http.StatusOK, azq)
}

// DeleteProjectQuota handles DELETE /organization/{orgaId}/project/{projectId}/quota
//
//	@Summary		Delete the project quota
//	@Description	Delete the quota and AZ quotas of a project and apply it to every AZ. Refused while the organization has a quota.
//	@Tags			v1, quota
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Success		204
//	@Failure		400
//	@Failure		403
//	@Failure		404
//	@Failure		409	"The organization has a quota"
//	@Failure		500
//	@Failure		502	"Deleted but not applied on some AZs (retry to apply it again)"
//	@Router			/v1/organization/{orgaId}/project/{projectId}/quota [delete]
//	@Security		Bearer[OrganizationRead, OrganizationProjectQuotaWrite]
func (h *Service) DeleteProjectQuota(w http.ResponseWriter, r *http.Request) {
	orgID, projID, ok := parseOrgAndProject(w, r)
	if !ok {
		return
	}
	if err := crudQuota.DeleteProjectQuota(r.Context(), orgID, projID); err != nil {
		WriteError(w, r, err, "failed to delete project quota")
		return
	}
	if err := quotasync.Project(r.Context(), orgID, projID); err != nil {
		WriteError(w, r, err, "failed to apply project quota")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseOrgAndProject(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	orgID, ok := ParseID(w, r, "orgaId")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	projID, ok := ParseID(w, r, "projectId")
	return orgID, projID, ok
}

// ParseID parses the UUID path parameter param, answering 400 when it is invalid.
func ParseID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msgf("invalid %s", param)
		return uuid.Nil, false
	}
	return id, true
}

// DecodeResources decodes a size-bounded quota request body, answering 400/413/415 on failure.
// The quantities themselves are validated by the crud layer.
func DecodeResources(w http.ResponseWriter, r *http.Request) (model.QuotaResources, bool) {
	var req model.QuotaResources
	if err := decoder.DecodeJSONFromHTTP(w, r, &req, maxBodySizeMB); err != nil {
		var decodeErr *decoder.HTTPJSONDecodeError
		if errors.As(err, &decodeErr) && decodeErr.Status != http.StatusInternalServerError {
			httpError.Http(w, r, decodeErr.Status).Msg(decodeErr.Message)
		} else {
			httpError.Http(w, r, http.StatusBadRequest).Msg("invalid request body")
		}
		return nil, false
	}
	if req == nil {
		req = model.QuotaResources{}
	}
	return req, true
}

// WriteError maps the business errors of the quota crud to an HTTP status. Any other error
// is logged and answered with a generic message, to not leak internal details.
func WriteError(w http.ResponseWriter, r *http.Request, err error, msg string) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, crudQuota.ErrInvalidQuantity),
		errors.Is(err, crudQuota.ErrOrgQuotaExceeded),
		errors.Is(err, crudQuota.ErrProjectQuotaExceeded),
		errors.Is(err, crudQuota.ErrBelowAllocated),
		errors.Is(err, crudQuota.ErrBelowUsage):
		status = http.StatusBadRequest
	case errors.Is(err, crudQuota.ErrUsageUnavailable),
		errors.Is(err, crudQuota.ErrAZUnavailable):
		// Logged by the crud layer; the message only holds the AZ code and resource name.
		status = http.StatusServiceUnavailable
	case errors.Is(err, crudQuota.ErrOrganizationNotFound),
		errors.Is(err, crudQuota.ErrProjectNotFound),
		errors.Is(err, crudQuota.ErrAZNotFound):
		status = http.StatusNotFound
	case errors.Is(err, crudQuota.ErrProjectQuotaEnforced):
		status = http.StatusConflict
	case errors.Is(err, quotasync.ErrNotApplied):
		// Logged by quotasync; the message only holds AZ codes.
		status = http.StatusBadGateway
	}
	if status == http.StatusInternalServerError {
		log := logger.GetLogger(r.Context())
		log.Err(err).Str("url", r.URL.Path).Msg(msg)
		httpError.Http(w, r, status).Msg(msg)
		return
	}
	httpError.Http(w, r, status).Msg(err.Error())
}

// WriteJSON answers body as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
