package paas

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/baas"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	spxIdMiddleware "github.com/super-phenix/superphenix/pkg/superphenix-id/middleware"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	"github.com/go-chi/chi/v5"
	"k8s.io/apimachinery/pkg/api/errors"
)

const baseBaasEndpoint = "/baas"

func BaasEndpoint(router chi.Router) {
	router.Route(baseBaasEndpoint, func(r chi.Router) {
		r.Get("/", listBaaS)
		r.Get("/check-limit", checkAllScopeLimit)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getBaaSByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getBaaSByEffectiveId)
			r.Delete("/", deleteBaaS)
		})
	})
}

// listBaaS
//
//	@Summary		Retrieve all baas
//	@Description	Retrieve all baas for a project
//	@Tags			v1, BaaS
//	@Produce		json
//	@Param			orgId		path	string		true	"Organization ID"
//	@Param			projectId	path	string		true	"Project ID"
//	@Success		200			{array}	view.BaaS	"BaaS List"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/baas [get]
func listBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	baasList, err := baas.ListBaaS(r.Context(), namespaceParam)
	if err != nil {
		log.Err(err).Msg("failed to retrieve resource")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}

	var resourceList []view.BaaS
	for _, item := range baasList {
		resource := view.BaaSToResource(item)
		resourceList = append(resourceList, resource)
	}

	b, _ := json.Marshal(resourceList)

	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getBaaSByLocalId
//
//	@Summary		Get baas by local ID
//	@Description	Get baas by local ID
//	@Tags			v1, BaaS
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			localId		path		string		true	"BaaS Local ID"
//	@Success		200			{object}	view.BaaS	"BaaS"
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/baas/localId/{localId} [get]
func getBaaSByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Ctx(r.Context()).Msg("failed to retrieve Resource Effective Id")
		http.Error(w, "failed to retrieve Resource Effective Id", http.StatusInternalServerError)
		return
	}
	getBaaS(w, r, effectiveId.(string))
}

// getBaaSByEffectiveId
//
//	@Summary		Get baas by effective ID
//	@Description	Get baas by effective ID
//	@Tags			v1, BaaS
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			effectiveId	path		string		true	"BaaS Effective ID"
//	@Success		200			{object}	view.BaaS	"BaaS"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/baas/{effectiveId} [get]
func getBaaSByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")
	getBaaS(w, r, effectiveId)
}

func getBaaS(w http.ResponseWriter, r *http.Request, effectiveId string) {
	log := logger.GetLogger(r.Context())

	namespaceParam := utils.GetRequestNamespace(r)
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	backup, err := baas.GetBaaS(r.Context(), namespaceParam, effectiveId)
	if errors.IsNotFound(err) {
		log.Err(err).Msg("Resource not found")
		httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
		return
	}
	if err != nil {
		log.Err(err).Msg("failed to retrieve resource")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}

	clusterResource := view.BaaSToResource(backup)

	b, _ := json.Marshal(clusterResource)

	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// checkAllScopeLimit
//
//	@Summary		Check all scope backup limit
//	@Description	Check if a new all-scoped backup can be created for a project
//	@Tags			v1, BaaS
//	@Produce		json
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			projectId	path		string	true	"Project ID"
//	@Success		200			{object}	object	"{canCreate: bool}"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/baas/check-limit [get]
func checkAllScopeLimit(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)

	canCreate, err := baas.CanCreateAllScopedBackup(r.Context(), namespaceParam)

	if err != nil {
		log.Err(err).Msg("failed to check all scope limit")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to check all scope limit")
	}

	result := struct {
		CanCreate bool `json:"canCreate"`
	}{CanCreate: canCreate}

	b, _ := json.Marshal(result)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// deleteBaaS
//
//	@Summary		Delete a baas
//	@Description	Delete a baas by effective ID
//	@Tags			v1, BaaS
//	@Produce		json
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"BaaS Effective ID"
//	@Success		204
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/baas/{effectiveId} [delete]
func deleteBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := chi.URLParam(r, "effectiveId")

	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	err := baas.DeleteBaaS(r.Context(), utils.GetRequestNamespace(r), effectiveId)
	if errors.IsNotFound(err) {
		log.Err(err).Msg("Resource not found")
		httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
		return
	}
	if err != nil {
		log.Err(err).Msg("failed to delete resource")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to delete resource")
		return
	}

	w.WriteHeader(http.StatusOK)
}
