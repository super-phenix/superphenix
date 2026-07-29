package kubeovn

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/vpc"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	spxIdMiddleware "github.com/super-phenix/superphenix/pkg/superphenix-id/middleware"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	"github.com/go-chi/chi/v5"
	"k8s.io/apimachinery/pkg/api/errors"
)

const baseVPCEndpoint = "/vpc"

func VPCEndpoint(router chi.Router) {
	router.Route(baseVPCEndpoint, func(r chi.Router) {
		r.Get("/", listVPCs)
		r.Post("/", createVPC)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getVPCByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getVPCByEffectiveId)

			r.Post("/", updateVPC)
			r.Delete("/", deleteVPC)
		})
	})
}

// listVPCs
//
//	@Summary		Retrieve all VPCs
//	@Description	Retrieve all VPCs for a project
//	@Tags			v1, VPC
//	@Produce		json
//	@Param			orgId		path	string		true	"Organization ID"
//	@Param			projectId	path	string		true	"Project ID"
//	@Success		200			{array}	view.VPC	"VPCs"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/vpc [get]
func listVPCs(w http.ResponseWriter, r *http.Request) {
	namespaceParam := utils.GetRequestNamespace(r)

	vpcViews := vpc.ListVPC(r.Context(), namespaceParam)

	var vpcs []view.VPC
	for _, vpcView := range vpcViews {
		s := view.VPCToResource(vpcView)
		vpcs = append(vpcs, s)
	}

	b, _ := json.Marshal(vpcs)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getVPCByLocalId
//
//	@Summary		Get VPC by local ID
//	@Description	Get VPC by local ID
//	@Tags			v1, VPC
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			localId		path		string		true	"VPC Local ID"
//	@Success		200			{object}	view.VPC	"VPC"
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/vpc/localId/{localId} [get]
func getVPCByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Msg("failed to retrieve Resource Effective Id")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource effective Id")
		return
	}
	getVPC(w, r, effectiveId.(string))
}

// getVPCByEffectiveId
//
//	@Summary		Get VPC by effective ID
//	@Description	Get VPC by effective ID
//	@Tags			v1, VPC
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			effectiveId	path		string		true	"VPC Effective ID"
//	@Success		200			{object}	view.VPC	"VPC"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/vpc/{effectiveId} [get]
func getVPCByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")
	getVPC(w, r, effectiveId)
}

func getVPC(w http.ResponseWriter, r *http.Request, effectiveId string) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	vpcView, err := vpc.GetVPC(r.Context(), namespaceParam, effectiveId)
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
	vpcResource := view.VPCToResource(vpcView)

	b, _ := json.Marshal(vpcResource)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// createVPC
//
//	@Summary		Create a VPC
//	@Description	Create a VPC
//	@Tags			v1, VPC
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Param			Body		body	vpc.CreateVPCInfo	true	"VPC info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/vpc [post]
func createVPC(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgId := chi.URLParam(r, "orgId")
	projectId := chi.URLParam(r, "projectId")

	err := k8s.CreateNamespaceIfNotExists(r.Context(), orgId, projectId)
	if err != nil {
		log.Error().Msg("Failed to create namespace")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create namespace")
		return
	}

	var body vpc.CreateVPCInfo
	err = decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	err = body.CreateVPC(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create vpc")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create vpc")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// updateVPC
//
//	@Summary		Update a VPC
//	@Description	Update a VPC
//	@Tags			v1, VPC
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Param			Body		body	vpc.UpdateVPCInfo	true	"VPC info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/vpc [post]
func updateVPC(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	var body vpc.UpdateVPCInfo
	if err := decoder.HandleHTTPJSON(w, r, &body, 5); err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	if err := body.UpdateVPC(r.Context(), namespaceParam, effectiveId); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		log.Error().Err(err).Msg("Failed to update vpc")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to update vpc")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// deleteVPC
//
//	@Summary		Delete a VPC
//	@Description	Delete a VPC by Effective ID
//	@Tags			v1, VPC
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"VPC Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/vpc [delete]
func deleteVPC(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	if err := vpc.DeleteVPC(r.Context(), namespaceParam, effectiveId); err != nil {
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
