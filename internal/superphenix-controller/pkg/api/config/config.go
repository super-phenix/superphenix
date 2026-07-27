package config

import (
	"encoding/json"
	"net/http"

	kovm "github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubevirt/vm"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubevirt/vmClusterPreference"
	_ "github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	"github.com/go-chi/chi/v5"
)

type KaaSConfig struct {
	StorageClasses []StorageClass `json:"storageClasses"`
}
type StorageClass struct {
	Shortname string `json:"name"`
	Fullname  string `json:"fullname"`
}

// storageClassKeys returns the friendly names from the unified StorageClassMapping.
func storageClassKeys() []string {
	keys := make([]string, 0, len(config.Global.StorageClassMapping))
	for k := range config.Global.StorageClassMapping {
		keys = append(keys, k)
	}
	return keys
}

// GetStorageClass
//
//	@Summary		Get all Storage Class
//	@Description	Get all Storage Class
//	@Tags			v1, Config
//	@Produce		json
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Success		200			{array}	string	"Storage Class"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/storage-class [get]
//	@Security		Bearer
func GetStorageClass(w http.ResponseWriter, r *http.Request) {
	b, _ := json.Marshal(storageClassKeys())
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// GetVMClusterPreference
//
//	@Summary		Get all VM Cluster Preference
//	@Description	Get all VM Cluster Preference
//	@Tags			v1, Config
//	@Produce		json
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Success		200			{array}	string	"VM Cluster Preference"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/vm-type [get]
//	@Security		Bearer
func GetVMClusterPreference(w http.ResponseWriter, r *http.Request) {
	res, err := vmClusterPreference.ListVMClusterPreference(r.Context())
	if err != nil {
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Error getting VM Cluster Preference")
		return
	}
	b, _ := json.Marshal(res)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// GetContainerDiskCatalog
//
//	@Summary		Get the container-disk catalog
//	@Description	Returns this AZ's catalog of container-disk types declared in the controller config.
//	@Tags			v1, Config
//	@Produce		json
//	@Param			orgId		path	string								true	"Organization ID"
//	@Param			projectId	path	string								true	"Project ID"
//	@Success		200			{array}	config.ContainerDiskCatalogEntry	"Catalog"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/container-disks [get]
//	@Security		Bearer
func GetContainerDiskCatalog(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(kovm.CatalogList())
	if err != nil {
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Error getting container disk catalog")
		return
	}
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// GetVMClusterPreferenceByName
//
//	@Summary		Get a VM Cluster Preference by name
//	@Description	Get a VM Cluster Preference by name
//	@Tags			v1, Config
//	@Produce		json
//	@Param			orgId		path		string								true	"Organization ID"
//	@Param			projectId	path		string								true	"Project ID"
//	@Param			name		path		string								true	"VM Cluster Preference Name"
//	@Success		200			{object}	view.VirtualMachinePreferenceView	"VM Cluster Preference"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/vm-type/{name} [get]
//	@Security		Bearer
func GetVMClusterPreferenceByName(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	res, err := vmClusterPreference.GetVMClusterPreference(r.Context(), name)
	if err != nil {
		httpError.Http(w, r, http.StatusNotFound).Msgf("VM Cluster Preference %s not found", name)
		return
	}
	b, _ := json.Marshal(res)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// GetVMClusterPreferenceAdvancedOptions
//
//	@Summary		Get the advanced options a VM Cluster Preference would apply
//	@Description	Resolve the advanced device/firmware options defaulted by a preference (no VM), for the create form. Sources are "preference"/"default".
//	@Tags			v1, Config
//	@Produce		json
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			projectId	path		string					true	"Project ID"
//	@Param			name		path		string					true	"VM Cluster Preference Name"
//	@Success		200			{object}	view.AdvancedOptions	"Advanced options"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/vm-type/{name}/advanced-options [get]
//	@Security		Bearer
func GetVMClusterPreferenceAdvancedOptions(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	namespace := utils.GetRequestNamespace(r)
	res, err := kovm.GetPreferenceAdvancedOptions(r.Context(), namespace, name)
	if err != nil {
		httpError.Http(w, r, http.StatusNotFound).Msgf("VM Cluster Preference %s not found", name)
		return
	}
	b, _ := json.Marshal(res)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// GetKaaSConfig
//
//	@Summary		Get KaaS Config
//	@Description	Get KaaS Configuration value
//	@Tags			v1, Config
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Success		200			{object}	KaaSConfig	"KaaS Config"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/kaas-config [get]
//	@Security		Bearer
func GetKaaSConfig(w http.ResponseWriter, r *http.Request) {
	storageClasses := make([]StorageClass, 0)
	for k, fullname := range config.Global.StorageClassMapping {
		storageClasses = append(storageClasses, StorageClass{
			Shortname: k,
			Fullname:  fullname,
		})
	}

	kaasConfig := KaaSConfig{
		StorageClasses: storageClasses,
	}

	b, _ := json.Marshal(kaasConfig)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}
