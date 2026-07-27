package argo

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/argo-controller/internal/utils"
	appProject "github.com/super-phenix/superphenix/internal/argo-controller/internal/v1/app_project"
	argoApp "github.com/super-phenix/superphenix/internal/argo-controller/internal/v1/argo_app"
	"github.com/super-phenix/superphenix/internal/argo-controller/internal/v1/k8s"
	"github.com/super-phenix/superphenix/internal/argo-controller/internal/v1/models/view"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	"github.com/go-chi/chi/v5"
	"k8s.io/apimachinery/pkg/api/errors"
)

const baseEndpoint = "/argo"

func InitRouter(router chi.Router) {
	router.Route(baseEndpoint, func(r chi.Router) {
		r.Get("/", listArgo)
		r.Post("/", createArgo)

		r.Get("/localId/{localId}", getArgoByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getArgoByEffectiveId)

			r.Post("/", updateArgo)
			r.Delete("/", deleteArgo)
		})
	})
}

// listArgo
//
//	@Summary		List Argo Apps
//	@Description	List every argo apps for a project
//	@Tags			v1, Argo
//	@Produce		json
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Success		200			{array}	view.Application	"Applications"
//	@Failure		400
//	@Failure		500
//	@Router			/v1/{orgId}/{projectId}/argo [get]
//	@Security		Bearer[]
func listArgo(w http.ResponseWriter, r *http.Request) {
	namespaceParam := utils.GetRequestNamespace(r)

	appViews := argoApp.ListApps(r.Context(), namespaceParam)

	var apps []view.Application
	for _, appView := range appViews {
		s := view.AppToResource(appView)
		apps = append(apps, s)
	}

	b, _ := json.Marshal(apps)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// createArgo
//
//	@Summary		Create an Argo App
//	@Description	Create an Argo App
//	@Tags			v1, Argo
//	@Produce		json
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			projectId	path	string					true	"Project ID"
//	@Param			Body		body	argoApp.CreateAppInfo	true	"Argo App"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/v1/{orgId}/{projectId}/argo [post]
//	@Security		Bearer[]
func createArgo(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgId := chi.URLParam(r, "orgId")
	projectId := chi.URLParam(r, "projectId")

	var body argoApp.CreateAppInfo
	if err := decoder.HandleHTTPJSON(w, r, &body, 5); err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	if err := k8s.CreateNamespaceIfNotExists(r.Context(), orgId, projectId); err != nil {
		log.Error().Msg("Failed to create namespace")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create namespace")
		return
	}

	if err := appProject.CreateAppProjectIfNotExists(r.Context(), orgId, projectId); err != nil {
		log.Error().Msg("Failed to create App Project")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create App Project")
		return

	}

	if err := body.CreateApp(r.Context()); err != nil {
		log.Error().Err(err).Msg("Failed to create argo app")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create argo app")
	} else {
		w.WriteHeader(http.StatusOK)
	}
	return
}

// getArgoByLocalId
//
//	@Summary		Get Argo App by Local ID
//	@Description	Get Argo App by Local ID
//	@Tags			v1, Argo
//	@Produce		json
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			localId		path		string				true	"Local ID"
//	@Success		200			{object}	view.Application	"Application"
//	@Failure		400
//	@Failure		500
//	@Router			/v1/{orgId}/{projectId}/argo/localId/{localId} [get]
//	@Security		Bearer[]
func getArgoByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Ctx(r.Context()).Msg("failed to retrieve Resource Effective Id")
		http.Error(w, "failed to retrieve Resource Effective Id", http.StatusInternalServerError)
		return
	}
	getArgo(w, r, effectiveId.(string))
}

// getArgoByEffectiveId
//
//	@Summary		Get Argo App by Effective ID
//	@Description	Get Argo App by Effective ID
//	@Tags			v1, Argo
//	@Produce		json
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			effectiveId	path		string				true	"Effective ID"
//	@Success		200			{object}	view.Application	"Application"
//	@Failure		400
//	@Failure		500
//	@Router			/v1/{orgId}/{projectId}/argo/{effectiveId} [get]
//	@Security		Bearer[]
func getArgoByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")
	getArgo(w, r, effectiveId)
}

func getArgo(w http.ResponseWriter, r *http.Request, effectiveId string) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	appView, err := argoApp.GetArgoApp(r.Context(), effectiveId, namespaceParam)
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
	appResource := view.AppToResource(appView)

	b, _ := json.Marshal(appResource)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// updateArgo
//
//	@Summary		Update Argo App
//	@Description	Update Argo App
//	@Tags			v1, Argo
//	@Produce		json
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			projectId	path	string					true	"Project ID"
//	@Param			effectiveId	path	string					true	"Effective ID"
//	@Param			Body		body	argoApp.UpdateAppInfo	true	"Argo App"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/v1/{orgId}/{projectId}/argo/{effectiveId} [post]
//	@Security		Bearer[]
func updateArgo(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	var body argoApp.UpdateAppInfo
	err := decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	err = body.UpdateApp(r.Context(), effectiveId, namespaceParam)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update argo app")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to update argo app")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// deleteArgo
//
//	@Summary		Delete Argo App
//	@Description	Delete Argo App by Effective ID
//	@Tags			v1, Argo
//	@Produce		json
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/v1/{orgId}/{projectId}/argo/{effectiveId} [delete]
//	@Security		Bearer[]
func deleteArgo(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	if err := argoApp.DeleteApp(r.Context(), effectiveId, namespaceParam); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Failed to delete resource - Not found")
			httpError.Http(w, r, http.StatusNotFound).Msg(http.StatusText(http.StatusNotFound))
		} else {
			log.Err(err).Msg("Failed to delete resource")
			httpError.Http(w, r, http.StatusInternalServerError).Msg(http.StatusText(http.StatusInternalServerError))
		}
	} else {
		w.WriteHeader(http.StatusOK)
	}
}
