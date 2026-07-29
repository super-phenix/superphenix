package kubeovn

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/eip"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/eip/dnat"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/eip/fip"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/eip/snat"
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

const baseEipEndpoint = "/eip"

func EipEndpoint(router chi.Router) {
	router.Route(baseEipEndpoint, func(r chi.Router) {
		r.Get("/", listEIPs)
		r.Post("/", createEIP)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getEIPByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getEIPByEffectiveId)

			r.Post("/", updateEIP)
			r.Delete("/", deleteEIP)
		})
	})
}

// listEIPs
//
//	@Summary		Retrieve all EIPs
//	@Description	Retrieve all EIPs for a project
//	@Tags			v1, EIP
//	@Produce		json
//	@Param			orgId		path	string		true	"Organization ID"
//	@Param			projectId	path	string		true	"Project ID"
//	@Success		200			{array}	view.EIP	"EIPs"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/eip [get]
func listEIPs(w http.ResponseWriter, r *http.Request) {
	namespaceParam := utils.GetRequestNamespace(r)

	eipList := eip.ListEips(namespaceParam)

	var eips []view.EIP
	for _, eipItem := range eipList {
		eipR := view.EIPViewToResource(eipItem, nil, nil, nil)
		eips = append(eips, eipR)
	}

	b, _ := json.Marshal(eips)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getEIPByLocalId
//
//	@Summary		Get EIP by local ID
//	@Description	Get EIP by local ID
//	@Tags			v1, EIP
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			localId		path		string		true	"EIP Local ID"
//	@Success		200			{object}	view.EIP	"EIP"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/eip/localId/{localId} [get]
func getEIPByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Msg("failed to retrieve Resource Effective Id")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource effective Id")
		return
	}

	getEIP(w, r, effectiveId.(string))
}

// getEIPByEffectiveId
//
//	@Summary		Get EIP by effective ID
//	@Description	Get EIP by effective ID
//	@Tags			v1, EIP
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			effectiveId	path		string		true	"EIP Effective ID"
//	@Success		200			{object}	view.EIP	"EIP"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/eip/{effectiveId} [get]
func getEIPByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")

	getEIP(w, r, effectiveId)
}

func getEIP(w http.ResponseWriter, r *http.Request, effectiveId string) {
	log := logger.GetLogger(r.Context())
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	eipView, err := eip.GetEip(r.Context(), utils.GetRequestNamespace(r), effectiveId)
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

	fipView, err := fip.Get(r.Context(), effectiveId)
	if err != nil {
		log.Err(err).Msg("failed to retrieve attached FIP")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve attached FIP")
		return
	}

	projectId := chi.URLParam(r, "projectId")
	snatList, err := snat.List(r.Context(), spxId.ToSPXID(projectId), effectiveId)
	if err != nil {
		log.Err(err).Msg("failed to retrieve attached SNAT")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve attached SNAT")
		return
	}

	dnatList, err := dnat.List(r.Context(), spxId.ToSPXID(projectId), effectiveId)
	if err != nil {
		log.Err(err).Msg("failed to retrieve attached DNAT")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve attached DNAT")
		return
	}

	eipResource := view.EIPViewToResource(eipView, fipView, snatList, dnatList)

	b, _ := json.Marshal(eipResource)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// createEIP
//
//	@Summary		Create an EIP
//	@Description	Create an EIP
//	@Tags			v1, EIP
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Param			Body		body	eip.CreateEIPInfo	true	"EIP info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/eip [post]
func createEIP(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgId := chi.URLParam(r, "orgId")
	projectId := chi.URLParam(r, "projectId")

	err := k8s.CreateNamespaceIfNotExists(r.Context(), orgId, projectId)
	if err != nil {
		log.Error().Msg("Failed to create namespace")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create namespace")
		return
	}

	var body eip.CreateEIPInfo
	err = decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	err = body.CreateEip(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create EIP")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create EIP")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// updateEIP
//
//	@Summary		Update an EIP
//	@Description	Update an EIP
//	@Tags			v1, EIP
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Param			Body		body	eip.UpdateEIPInfo	true	"EIP info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/eip [post]
func updateEIP(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	var body eip.UpdateEIPInfo
	err := decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	err = body.UpdateEip(r.Context(), utils.GetRequestNamespace(r), effectiveId)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		log.Error().Err(err).Msg("Failed to update EIP")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to update EIP")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// deleteEIP
//
//	@Summary		Delete an EIP
//	@Description	Delete an EIP by Effective ID - a 400ms delay has been added to avoid a KubeOVN error (caused by the FIP not being deleted when we try to delete the EIP)
//	@Tags			v1, EIP
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"EIP Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/eip [delete]
func deleteEIP(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	if err := eip.DeleteEip(r.Context(), namespaceParam, effectiveId); err != nil {
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
