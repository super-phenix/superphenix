package permission

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authorization"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
	pw "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/permission"

	"github.com/go-chi/chi/v5"
)

type ListPermissionBody struct {
	EntityType string `json:"entityType"`
	EntityId   string `json:"entityId"`
}

// ListPermissions Returns the list of permissions held by the logged-in user for an entity
//
//	@Summary		List logged-in user permissions
//	@Description	Returns the list of permissions held by the logged-in user for an entity
//	@Tags			v1, iam
//	@Accept			json
//	@Produce		json
//	@Param			orgaId	path	string				true	"Organization ID"
//	@Param			Body	body	ListPermissionBody	true	"Entity Info"
//	@Success		200		{array}	string				"Permissions List"
//	@Failure		400
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/iam/permissions [post]
//	@Security		Bearer[OrganizationRead]
func (h *Service) ListPermissions(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	userId := r.Context().Value(consts.ContextUserId)
	orgaId := chi.URLParam(r, "orgaId")

	if userId == nil {
		log.Error().Msg("No id found in context")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var body ListPermissionBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	canBypass := authorization.IsSuperAdminUser(userId.(string))

	if canBypass {
		log.Info().Ctx(r.Context()).
			Str("method", "ListPermissions").
			Str("url", r.URL.String()).
			Str("userId", userId.(string)).
			Msg("Return full permission list")
	}

	permList, err := pw.ListUserPermission(r.Context(), orgaId, body.EntityType, body.EntityId, userId.(string))
	if !canBypass && err != nil {
		log.Error().Err(err).Msg("Failed to list permissions")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if canBypass {
		permList = make([]string, 0)
		for i, k := range permission.PermissionsEntityMap {
			if k == body.EntityType {
				permList = append(permList, i)
			}
		}
	}

	marshal, err := json.Marshal(permList)
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling response")
	}
	ch.Data(w, http.StatusOK, ch.MIMEJSON, marshal)
}
