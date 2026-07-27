package paas

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kaas"
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

const baseKaasEndpoint = "/kaas"

func KaaSEndpoint(router chi.Router) {
	router.Route(baseKaasEndpoint, func(r chi.Router) {
		r.Get("/", listClusters)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getClusterByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getClusterByEffectiveId)
			r.Get("/instances", listClusterInstances)
			r.Get("/netpols", listClusterNetPols)
			r.Post("/delete-group", deleteNodeGroup)
			r.Get("/kubeconfig", getKubeConfig)
		})
	})
}

// listClusters
//
//	@Summary		Retrieve all clusters
//	@Description	Retrieve all clusters for a project
//	@Tags			v1, Cluster
//	@Produce		json
//	@Param			orgId		path	string		true	"Organization ID"
//	@Param			projectId	path	string		true	"Project ID"
//	@Success		200			{array}	view.KaaS	"Clusters"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/kaas [get]
func listClusters(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	clusters, err := kaas.ListCluster(r.Context(), namespaceParam)
	if err != nil {
		log.Err(err).Msg("failed to retrieve resource")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}

	var resourceList []view.KaaS
	for _, cluster := range clusters {
		resource := view.KaaSToResource(cluster)
		resourceList = append(resourceList, resource)
	}

	b, _ := json.Marshal(resourceList)

	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getClusterByLocalId
//
//	@Summary		Get cluster by local ID
//	@Description	Get cluster by local ID
//	@Tags			v1, Cluster
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			localId		path		string		true	"Cluster Local ID"
//	@Success		200			{object}	view.KaaS	"Cluster"
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/kaas/localId/{localId} [get]
func getClusterByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Ctx(r.Context()).Msg("failed to retrieve Resource Effective Id")
		http.Error(w, "failed to retrieve Resource Effective Id", http.StatusInternalServerError)
		return
	}
	getCluster(w, r, effectiveId.(string))
}

// getClusterByEffectiveId
//
//	@Summary		Get cluster by effective ID
//	@Description	Get cluster by effective ID
//	@Tags			v1, Cluster
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			effectiveId	path		string		true	"Cluster Effective ID"
//	@Success		200			{object}	view.KaaS	"Cluster"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/kaas/{effectiveId} [get]
func getClusterByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")
	getCluster(w, r, effectiveId)
}

func getCluster(w http.ResponseWriter, r *http.Request, effectiveId string) {
	log := logger.GetLogger(r.Context())

	namespaceParam := utils.GetRequestNamespace(r)
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	cluster, err := kaas.GetCluster(r.Context(), namespaceParam, effectiveId)
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

	clusterResource := view.KaaSToResource(cluster)

	b, _ := json.Marshal(clusterResource)

	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// listClusterInstances
//
//	@Summary		List cluster instances
//	@Description	List cluster instances (VMs and VMIs)
//	@Tags			v1, Cluster
//	@Produce		json
//	@Param			orgId		path	string			true	"Organization ID"
//	@Param			projectId	path	string			true	"Project ID"
//	@Param			effectiveId	path	string			true	"Cluster Effective ID"
//	@Success		200			{array}	view.Instance	"Instances"
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/kaas/{effectiveId}/instances [get]
func listClusterInstances(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	instances, err := kaas.GetClusterMachines(r.Context(), namespaceParam, effectiveId)
	if err != nil {
		log.Err(err).Msg("failed to retrieve resource")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}
	b, _ := json.Marshal(instances)

	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// listClusterNetPols
//
//	@Summary		List cluster network policies
//	@Description	List cluster network policies
//	@Tags			v1, Cluster
//	@Produce		json
//	@Param			orgId		path	string			true	"Organization ID"
//	@Param			projectId	path	string			true	"Project ID"
//	@Param			effectiveId	path	string			true	"Cluster Effective ID"
//	@Success		200			{array}	view.Firewall	"NetPols"
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/kaas/{effectiveId}/netpols [get]
func listClusterNetPols(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	netpols, err := kaas.GetNetPols(r.Context(), namespaceParam, effectiveId)
	if err != nil {
		log.Err(err).Msg("failed to retrieve resource")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}
	b, _ := json.Marshal(netpols)

	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// deleteNodeGroup
//
// Note: This method has been implemented because ArgoCD cannot delete old MachineDeployment due
// to OwnerReference in MachineDeployment.
// We DO NOT recommend to call this endpoint in any other case than cleaning old MachineDeployment.
//
//	@Summary		Delete a node group
//	@Description	Delete a node group by cluster effective ID and group name
//	@Tags			v1, Cluster
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string			true	"Organization ID"
//	@Param			projectId	path	string			true	"Project ID"
//	@Param			effectiveId	path	string			true	"Cluster Effective ID"
//	@Param			Body		body	GroupDeletion	true	"Group deletion info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/kaas/{effectiveId}/delete-group [post]
func deleteNodeGroup(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	var body GroupDeletion
	err := decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	if err := kaas.DeleteGroup(r.Context(), namespaceParam, effectiveId, body.GroupName); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		log.Err(err).Msg("failed to delete machine deployment")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to delete machine deployment")
		return
	}
}

// getKubeConfig
//
//	@Summary		Get cluster kubeconfig
//	@Description	Get cluster kubeconfig file content
//	@Tags			v1, Cluster
//	@Produce		octet-stream
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Cluster Effective ID"
//	@Success		200			{file}	binary	"KubeConfig file"
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/kaas/{effectiveId}/kubeconfig [get]
func getKubeConfig(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	kubeconfig, err := kaas.GetKubeConfig(r.Context(), namespaceParam, effectiveId)
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

	http.ServeContent(w, r, ".kubeconfig", time.Time{}, bytes.NewReader(kubeconfig))
}
