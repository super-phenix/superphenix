package kubeovn

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s"
	lb "github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/load_balancer"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/go-chi/chi/v5"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	spxIdMiddleware "github.com/super-phenix/superphenix/pkg/superphenix-id/middleware"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	"k8s.io/apimachinery/pkg/api/errors"
)

const baseBalancerEndpoint = "/load-balancer"

func LoadBalancerEndpoint(router chi.Router) {
	router.Route(baseBalancerEndpoint, func(r chi.Router) {
		r.Get("/", listLoadBalancers)
		r.Post("/", createLoadBalancer)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getLoadBalancerByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getLoadBalancerByEffectiveId)

			r.Post("/", updateLoadBalancer)
			r.Delete("/", deleteLoadBalancer)
		})
	})
}

// listLoadBalancers
//
//	@Summary		Retrieve all LoadBalancers
//	@Description	Retrieve all LoadBalancers for a project
//	@Tags			v1, LoadBalancer
//	@Produce		json
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Success		200			{array}	view.LoadBalancer	"LoadBalancers"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/load-balancer [get]
func listLoadBalancers(w http.ResponseWriter, r *http.Request) {
	namespaceParam := utils.GetRequestNamespace(r)

	lbViews := lb.ListLoadBalancer(r.Context(), namespaceParam)

	var lbs []view.LoadBalancer
	for _, lbView := range lbViews {
		s := lbView.ToResource()
		lbs = append(lbs, s)
	}

	b, _ := json.Marshal(lbs)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getLoadBalancerByLocalId
//
//	@Summary		Get LoadBalancer by local ID
//	@Description	Get LoadBalancer by local ID
//	@Tags			v1, LoadBalancer
//	@Produce		json
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			localId		path		string				true	"LoadBalancer Local ID"
//	@Success		200			{object}	view.LoadBalancer	"LoadBalancer"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/load-balancer/localId/{localId} [get]
func getLoadBalancerByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Msg("failed to retrieve Resource Effective Id")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource effective Id")
		return
	}
	getLoadBalancer(w, r, effectiveId.(string))
}

// getLoadBalancerByEffectiveId
//
//	@Summary		Get LoadBalancer by effective ID
//	@Description	Get LoadBalancer by effective ID
//	@Tags			v1, LoadBalancer
//	@Produce		json
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			effectiveId	path		string				true	"LoadBalancer Effective ID"
//	@Success		200			{object}	view.LoadBalancer	"LoadBalancer"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/load-balancer/{effectiveId} [get]
func getLoadBalancerByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")
	getLoadBalancer(w, r, effectiveId)
}

func getLoadBalancer(w http.ResponseWriter, r *http.Request, effectiveId string) {
	log := logger.GetLogger(r.Context())
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	lbView, err := lb.GetLoadBalancer(r.Context(), utils.GetRequestNamespace(r), effectiveId)
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
	lbResource := lbView.ToResource()

	b, _ := json.Marshal(lbResource)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// createLoadBalancer
//
//	@Summary		Create an LoadBalancer
//	@Description	Create an LoadBalancer
//	@Tags			v1, LoadBalancer
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string						true	"Organization ID"
//	@Param			projectId	path	string						true	"Project ID"
//	@Param			Body		body	lb.CreateLoadBalancerInfo	true	"LoadBalancer info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/load-balancer [post]
func createLoadBalancer(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgId := chi.URLParam(r, "orgId")
	projectId := chi.URLParam(r, "projectId")

	err := k8s.CreateNamespaceIfNotExists(r.Context(), orgId, projectId)
	if err != nil {
		log.Error().Msg("Failed to create namespace")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create namespace")
		return
	}

	var body lb.CreateLoadBalancerInfo
	err = decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	err = body.CreateLoadBalancer(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create load balancer")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create load balancer")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// updateLoadBalancer
//
//	@Summary		Update a LoadBalancer
//	@Description	Update a LoadBalancer
//	@Tags			v1, LoadBalancer
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string						true	"Organization ID"
//	@Param			projectId	path	string						true	"Project ID"
//	@Param			Body		body	lb.CreateLoadBalancerInfo	true	"LoadBalancer info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/load-balancer/{effectiveId} [post]
func updateLoadBalancer(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	var body lb.UpdateLoadBalancerInfo
	err := decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	err = body.UpdateLoadBalancer(r.Context(), utils.GetRequestNamespace(r), effectiveId)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		log.Error().Err(err).Msg("Failed to update load balancer")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to update load balancer")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// deleteLoadBalancer
//
//	@Summary		Delete an LoadBalancer
//	@Description	Delete an LoadBalancer by Effective ID
//	@Tags			v1, LoadBalancer
//	@Produce		plain
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			effectiveId	path		string	true	"LoadBalancer Effective ID"
//	@Success		200			{string}	string	"deleted"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/load-balancer [delete]
func deleteLoadBalancer(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	if err := lb.DeleteLoadBalancer(r.Context(), utils.GetRequestNamespace(r), effectiveId); err != nil {
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
