package k8s

import (
	"encoding/json"
	stderrors "errors"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s/netpol"
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

const baseSecurityGroupEndpoint = "/security-group"

func SecurityGroupEndpoint(router chi.Router) {
	router.Route(baseSecurityGroupEndpoint, func(r chi.Router) {
		r.Get("/", listNetPols)
		r.Post("/", createNetPol)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getNetPolByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getNetPolByEffectiveId)

			r.Post("/", updateNetPol)
			r.Delete("/", deleteNetPol)
		})
	})
}

// listNetPols
//
//	@Summary		Retrieve all Network Policies
//	@Description	Retrieve all Network Policies for a project
//	@Tags			v1, NetworkPolicy
//	@Produce		json
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Success		200			{array}	view.SecurityGroup	"Network Policies"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/security-group [get]
func listNetPols(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)

	netPolList, err := netpol.ListNetPols(r.Context(), namespaceParam)
	if err != nil {
		log.Err(err).Msg("failed to retrieve resource")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}

	var netPols []view.SecurityGroup
	for _, netPolItem := range netPolList {
		netPolR := netPolItem.ToResource()
		netPols = append(netPols, netPolR)
	}

	b, _ := json.Marshal(netPols)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getNetPolByLocalId
//
//	@Summary		Get Network Policy by local ID
//	@Description	Get Network Policy by local ID
//	@Tags			v1, NetworkPolicy
//	@Produce		json
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			localId		path		string				true	"Network Policy Local ID"
//	@Success		200			{object}	view.SecurityGroup	"Network Policy"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/security-group/localId/{localId} [get]
func getNetPolByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Msg("failed to retrieve Resource Effective Id")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource effective Id")
		return
	}

	getNetPol(w, r, effectiveId.(string))
}

// getNetPolByEffectiveId
//
//	@Summary		Get Network Policy by effective ID
//	@Description	Get Network Policy by effective ID
//	@Tags			v1, NetworkPolicy
//	@Produce		json
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			effectiveId	path		string				true	"Network Policy Effective ID"
//	@Success		200			{object}	view.SecurityGroup	"Network Policy"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/security-group/{effectiveId} [get]
func getNetPolByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")

	getNetPol(w, r, effectiveId)
}

func getNetPol(w http.ResponseWriter, r *http.Request, effectiveId string) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	if namespaceParam == "" {
		log.Error().Msg("no namespace provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no project Id provided")
		return
	}

	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	netPolView, err := netpol.GetNetPol(r.Context(), namespaceParam, effectiveId)
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

	NetPolResource := netPolView.ToResource()

	b, _ := json.Marshal(NetPolResource)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// createNetPol
//
//	@Summary		Create a NetworkPolicy
//	@Description	Create a NetworkPolicy
//	@Tags			v1, NetworkPolicy
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			projectId	path	string					true	"Project ID"
//	@Param			Body		body	netpol.CreateNetPolInfo	true	"NetworkPolicy info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/security-group [post]
func createNetPol(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgId := chi.URLParam(r, "orgId")
	projectId := chi.URLParam(r, "projectId")
	namespace := utils.GetNamespace(projectId)

	err := k8s.CreateNamespaceIfNotExists(r.Context(), orgId, projectId)
	if err != nil {
		log.Error().Msg("Failed to create namespace")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create namespace")
		return
	}

	var body netpol.CreateNetPolInfo
	err = decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	err = body.CreateNetPol(r.Context(), namespace)
	if err != nil {
		if stderrors.Is(err, netpol.ErrUnknownSubnet) || stderrors.Is(err, netpol.ErrTooManySubnets) {
			log.Error().Err(err).Msg("Invalid subnet scope")
			httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
			return
		}
		log.Error().Err(err).Msg("Failed to create network policy")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create network policy")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// updateNetPol
//
//	@Summary		Update a Network Policy
//	@Description	Update a Network Policy
//	@Tags			v1, Network Policy
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			projectId	path	string					true	"Project ID"
//	@Param			Body		body	netpol.UpdateNetPolInfo	true	"network Policy info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/security-group/{effectiveId} [post]
func updateNetPol(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	projectId := chi.URLParam(r, "projectId")
	effectiveId := chi.URLParam(r, "effectiveId")
	namespace := utils.GetNamespace(projectId)
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	var body netpol.UpdateNetPolInfo
	err := decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	err = body.UpdateNetPol(r.Context(), namespace, effectiveId)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		if stderrors.Is(err, netpol.ErrUnknownSubnet) || stderrors.Is(err, netpol.ErrTooManySubnets) {
			log.Error().Err(err).Msg("Invalid subnet scope")
			httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
			return
		}
		log.Error().Err(err).Msg("Failed to update Network Policy")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to update Network Policy")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// deleteNetPol
//
//	@Summary		Delete a NetworkPolicy
//	@Description	Delete a NetworkPolicy by Effective ID
//	@Tags			v1, NetworkPolicy
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"NetworkPolicy Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/security-group [delete]
func deleteNetPol(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	if err := netpol.DeleteNetPol(r.Context(), namespaceParam, effectiveId); err != nil {
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
