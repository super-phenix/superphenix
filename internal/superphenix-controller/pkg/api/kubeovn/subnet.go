package kubeovn

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/eip"
	natGateway "github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/nat_gateway"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/subnet"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubevirt/vm"
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

const baseSubnetEndpoint = "/subnet"

func SubnetEndpoint(router chi.Router) {
	router.Route(baseSubnetEndpoint, func(r chi.Router) {
		r.Get("/", listSubnets)
		r.Post("/", createSubnet)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getSubnetByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getSubnetByEffectiveId)
			r.Get("/has-eip", hasEIP)

			r.Post("/", updateSubnet)
			r.Delete("/", deleteSubnet)
		})
	})
}

// listSubnets
//
//	@Summary		Retrieve all Subnets
//	@Description	Retrieve all Subnets for a project
//	@Tags			v1, Subnet
//	@Produce		json
//	@Param			orgId		path	string		true	"Organization ID"
//	@Param			projectId	path	string		true	"Project ID"
//	@Success		200			{array}	view.Subnet	"Subnets"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/subnet [get]
func listSubnets(w http.ResponseWriter, r *http.Request) {
	namespaceParam := utils.GetRequestNamespace(r)

	subnetViews := subnet.ListSubnet(r.Context(), namespaceParam)
	natGwViews := natGateway.ListNatGw(namespaceParam)

	natGwMap := make(map[string]*view.NatGwView)
	for _, natGwView := range natGwViews {
		natGwMap[natGwView.Spec.Subnet] = &natGwView
	}

	var subnets []view.Subnet
	for _, subnetView := range subnetViews {
		s := view.SubnetViewToResource(subnetView, natGwMap[subnetView.Name])
		subnets = append(subnets, s)
	}

	b, _ := json.Marshal(subnets)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getSubnetByLocalId
//
//	@Summary		Get Subnet by local ID
//	@Description	Get Subnet by local ID
//	@Tags			v1, Subnet
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			localId		path		string		true	"Subnet Local ID"
//	@Success		200			{object}	view.Subnet	"Subnet"
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/subnet/localId/{localId} [get]
func getSubnetByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Msg("failed to retrieve Resource Effective Id")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource effective Id")
		return
	}
	getSubnet(w, r, effectiveId.(string))
}

// getSubnetByEffectiveId
//
//	@Summary		Get Subnet by effective ID
//	@Description	Get Subnet by effective ID
//	@Tags			v1, Subnet
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			effectiveId	path		string		true	"Subnet Effective ID"
//	@Success		200			{object}	view.Subnet	"Subnet"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/subnet/{effectiveId} [get]
func getSubnetByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")
	getSubnet(w, r, effectiveId)
}

func getSubnet(w http.ResponseWriter, r *http.Request, effectiveId string) {
	log := logger.GetLogger(r.Context())
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	namespaceParam := utils.GetRequestNamespace(r)
	subnetView, err := subnet.GetSubnet(r.Context(), effectiveId, namespaceParam)
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

	natGw, err := natGateway.GetNatGw(r.Context(), namespaceParam, effectiveId)
	if err != nil {
		log.Err(err).Msg("failed to retrieve attached Nat Gateway")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve attached Nat Gateway")
		return
	}
	subnetResource := view.SubnetViewToResource(subnetView, natGw)

	b, _ := json.Marshal(subnetResource)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)

}

// hasEIP
//
//	@Summary		Check if a Subnet have linked EIP
//	@Description	Check if a Subnet have linked EIP
//	@Tags			v1, Subnet
//	@Produce		json
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			effectiveId	path		string				true	"Subnet Effective ID"
//	@Success		200			{object}	object{HasEIP bool}	"Result"
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/subnet/{effectiveId}/has-eip [get]
func hasEIP(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	// Check EIP
	eipExist, err := eip.IsEIPInSubnet(r.Context(), namespaceParam, effectiveId)
	if err != nil {
		log.Err(err).Str("namespace", namespaceParam).Str("name", effectiveId).Msg("Error checking subnet")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Error checking subnet")
		return
	}

	b, _ := json.Marshal(struct {
		HasEIP bool `json:"hasEIP"`
	}{HasEIP: eipExist})
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)

}

// createSubnet
//
//	@Summary		Create a Subnet
//	@Description	Create a Subnet
//	@Tags			v1, Subnet
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			projectId	path	string					true	"Project ID"
//	@Param			Body		body	subnet.CreateSubnetInfo	true	"Subnet info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/subnet [post]
func createSubnet(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgId := chi.URLParam(r, "orgId")
	projectId := chi.URLParam(r, "projectId")

	err := k8s.CreateNamespaceIfNotExists(r.Context(), orgId, projectId)
	if err != nil {
		log.Error().Msg("Failed to create namespace")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create namespace")
		return
	}

	var body subnet.CreateSubnetInfo
	err = decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	err = body.CreateSubnet(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create subnet")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create subnet")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// updateSubnet
//
//	@Summary		Update a Subnet
//	@Description	Update a Subnet
//	@Tags			v1, Subnet
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			projectId	path	string					true	"Project ID"
//	@Param			Body		body	subnet.CreateSubnetInfo	true	"Subnet info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/subnet/{effectiveId} [post]
func updateSubnet(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	var body subnet.UpdateSubnetInfo
	err := decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	err = body.UpdateSubnet(r.Context(), namespaceParam, effectiveId)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		log.Error().Err(err).Msg("Failed to update subnet")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to update subnet")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// deleteSubnet
//
//	@Summary		Delete a Subnet
//	@Description	Delete a Subnet by Effective ID
//	@Tags			v1, Subnet
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Subnet Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/subnet [delete]
func deleteSubnet(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	// Check VM
	vmExist, err := vm.IsVMInSubnet(r.Context(), namespaceParam, effectiveId)
	if err != nil {
		log.Err(err).Str("namespace", namespaceParam).Str("name", effectiveId).Msg("Error checking subnet")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Error checking subnet")
		return
	}
	if vmExist {
		log.Err(err).Str("namespace", namespaceParam).Str("name", effectiveId).Msg("Cannot delete used subnet")
		httpError.Http(w, r, http.StatusBadRequest).Msg("cannot delete used subnet")
		return
	}

	// Check EIP
	eipExist, err := eip.IsEIPInSubnet(r.Context(), namespaceParam, effectiveId)
	if err != nil {
		log.Error().Err(err).Str("namespace", namespaceParam).Str("name", effectiveId).Msg("Error checking subnet")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Error checking subnet")
		return
	}
	if eipExist {
		log.Error().Err(err).Str("namespace", namespaceParam).Str("name", effectiveId).Msg("Cannot delete used subnet")
		httpError.Http(w, r, http.StatusBadRequest).Msg("cannot delete used subnet")
		return
	}

	if err := subnet.DeleteSubnet(r.Context(), namespaceParam, effectiveId); err != nil {
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
