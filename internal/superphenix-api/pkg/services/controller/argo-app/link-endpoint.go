package argoApp

import (
	"encoding/json"
	"fmt"
	"net/http"

	ctrlutils "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/utils"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/go-chi/chi/v5"
)

type ArgoCdLinkResponse struct {
	Link string `json:"link"`
}

// GetArgoCdLink returns the Argo CD application link for a given resource
//
//	@Summary		Get Argo CD application link
//	@Description	Returns the Argo CD UI link for a KaaS, BaaS or IaaS resource
//	@Tags			v1, Argo CD
//	@Produce		json
//	@Param			orgaId		path		string				true	"Organization ID"
//	@Param			az			path		string				true	"AZ Code"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			kind		path		string				true	"Resource kind (kaas, baas, iaas)"
//	@Param			effectiveId	path		string				true	"Effective ID of the resource"
//	@Success		200			{object}	ArgoCdLinkResponse	"Argo CD Link"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/argo-link/{kind}/{effectiveId} [get]
//	@Security		Bearer[OrganizationRead, ProjectArgoCdRead]
func (h *Service) GetArgoCdLink(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())

	azDb, _, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	kind := chi.URLParam(r, "kind")
	effectiveId := chi.URLParam(r, "effectiveId")

	argoCdUrl := h.cfg.ArgoCdUrl

	var appName string
	switch kind {
	case KindKaaS:
		appName = fmt.Sprintf("%s-%s", KindKaaS, effectiveId)
	case KindBaaS:
		appName = fmt.Sprintf("%s-%s", KindBaaS, effectiveId)
	default:
		appName = fmt.Sprintf("iaas-%s", azDb.Destination)
	}

	link := fmt.Sprintf("%s/applications/%s-%s/%s", argoCdUrl, h.cfg.SpxPrefix, projectDb.ID, appName)

	response := ArgoCdLinkResponse{Link: link}
	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(response)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal response")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to build response.")
		return
	}
	_, _ = w.Write(marshal)
}
