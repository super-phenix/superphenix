package kubevirt

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	spxIdMiddleware "github.com/super-phenix/superphenix/pkg/superphenix-id/middleware"

	"github.com/go-chi/chi/v5"
	"k8s.io/apimachinery/pkg/api/errors"

	kovm "github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubevirt/vm"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"
)

const baseVMEndpoint = "/instance"

func VMEndpoint(router chi.Router) {
	router.Route(baseVMEndpoint, func(r chi.Router) {
		r.Get("/", listInstances)
		r.Post("/", createInstance)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getInstanceByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getInstanceByEffectiveId)
			r.Post("/", updateInstance)
			r.Delete("/", deleteInstance)

			r.Get("/start", startVM)
			r.Get("/stop", stopVM)
			r.Get("/stop-force", stopForceVM)
			r.Get("/restart", restartVM)

			r.Get("/advanced-options", getInstanceAdvancedOptions)

			r.Post("/container-disk/mount", mountContainerDisk)
			r.Post("/container-disk/unmount", unmountContainerDisk)
		})
	})
}

// listInstances
//
//	@Summary		Retrieve all instances
//	@Description	Retrieve all instances for a project
//	@Tags			v1, Instance
//	@Produce		json
//	@Param			orgId		path	string			true	"Organization ID"
//	@Param			projectId	path	string			true	"Project ID"
//	@Success		200			{array}	view.Instance	"Instances"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance [get]
func listInstances(w http.ResponseWriter, r *http.Request) {
	namespaceParam := utils.GetRequestNamespace(r)
	vms := kovm.ListVM(r.Context(), namespaceParam)
	vmis := kovm.ListVMI(r.Context(), namespaceParam)

	instances := view.VMsToResources(vms, vmis)
	b, _ := json.Marshal(instances)

	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getInstanceByLocalId
//
//	@Summary		Get instance by local ID
//	@Description	Get instance by local ID
//	@Tags			v1, Instance
//	@Produce		json
//	@Param			orgId		path		string			true	"Organization ID"
//	@Param			projectId	path		string			true	"Project ID"
//	@Param			localId		path		string			true	"Instance Local ID"
//	@Success		200			{object}	view.Instance	"Instance"
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance/localId/{localId} [get]
func getInstanceByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Ctx(r.Context()).Msg("failed to retrieve Resource Effective Id")
		http.Error(w, "failed to retrieve Resource Effective Id", http.StatusInternalServerError)
		return
	}
	getInstance(w, r, effectiveId.(string))
}

// getInstanceByEffectiveId
//
//	@Summary		Get instance by effective ID
//	@Description	Get instance by effective ID
//	@Tags			v1, Instance
//	@Produce		json
//	@Param			orgId		path		string			true	"Organization ID"
//	@Param			projectId	path		string			true	"Project ID"
//	@Param			effectiveId	path		string			true	"Instance Effective ID"
//	@Success		200			{object}	view.Instance	"Instance"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance/{effectiveId} [get]
func getInstanceByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")
	getInstance(w, r, effectiveId)
}

func getInstance(w http.ResponseWriter, r *http.Request, effectiveId string) {
	log := logger.GetLogger(r.Context())

	namespaceParam := utils.GetRequestNamespace(r)
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	vm, err := kovm.GetInfoVM(r.Context(), namespaceParam, effectiveId)
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
	vmi, err := kovm.GetInfoVMI(r.Context(), namespaceParam, effectiveId)

	instance := view.Instance{
		Resource: view.Resource{
			ID:          vm.Labels[spxId.SpxLabelResourceLocalID],
			EId:         vm.Name,
			ProductName: vm.Labels[spxId.SpxLabelResourceName],
			Gitops:      vm.Labels[spxId.SpxLabelGitops],
		},
		Vm: vm,
	}

	if err == nil && vmi.Name != "" {
		instance.Vmi = vmi
	}

	init, err := kovm.GetCloudInit(r.Context(), namespaceParam, effectiveId)
	if err != nil {
		log.Err(err).Msg("failed to retrieve cloud init")
	} else {
		instance.CloudInit = init
	}

	b, _ := json.Marshal(instance)

	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// startVM
//
//	@Summary		Start Instance VMI
//	@Description	Start Instance VMI
//	@Tags			v1, Instance
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Instance Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance/{effectiveId}/start [get]
func startVM(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespace, effectiveId, err := utils.RetrieveNamespaceAndEid(w, r)
	if err != nil {
		return
	}

	log.Info().Msgf("starting VM %s", effectiveId)
	if err := kovm.StartVM(r.Context(), namespace, effectiveId); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		log.Err(err).Msg("failed to start VM")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Error starting VM")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// stopVM
//
//	@Summary		Stop Instance VMI
//	@Description	Stop Instance VMI
//	@Tags			v1, Instance
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Instance Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance/{effectiveId}/stop [get]
func stopVM(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespace, effectiveId, err := utils.RetrieveNamespaceAndEid(w, r)
	if err != nil {
		return
	}

	log.Info().Msgf("stopping VM %s", effectiveId)
	if err := kovm.StopVM(r.Context(), namespace, effectiveId, false); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		log.Err(err).Msg("failed to stop VM")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Error stopping VM")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// stopForceVM
//
//	@Summary		Force Stop Instance VMI
//	@Description	Force Stop Instance VMI
//	@Tags			v1, Instance
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Instance Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance/{effectiveId}/stop-force [get]
func stopForceVM(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespace, effectiveId, err := utils.RetrieveNamespaceAndEid(w, r)
	if err != nil {
		return
	}

	log.Info().Msgf("force stopping VM %s", effectiveId)
	if err := kovm.StopVM(r.Context(), namespace, effectiveId, true); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		log.Err(err).Msg("failed to stop VM")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Error stopping VM")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// restartVM
//
//	@Summary		Restart Instance VMI
//	@Description	Restart Instance VMI
//	@Tags			v1, Instance
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Instance Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance/{effectiveId}/restart [get]
func restartVM(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespace, effectiveId, err := utils.RetrieveNamespaceAndEid(w, r)
	if err != nil {
		return
	}

	log.Info().Msgf("restart VM %s", effectiveId)
	if err := kovm.RestartVM(r.Context(), namespace, effectiveId); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		log.Err(err).Msg("failed to restart VM")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Error restarting VM")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// getInstanceAdvancedOptions
//
//	@Summary		Get an instance's advanced options
//	@Description	Resolve the effective advanced device/firmware options of an instance (value + source: vm/preference/default).
//	@Tags			v1, Instance
//	@Produce		json
//	@Param			orgId		path		string					true	"Organization ID"
//	@Param			projectId	path		string					true	"Project ID"
//	@Param			effectiveId	path		string					true	"Instance Effective ID"
//	@Success		200			{object}	view.AdvancedOptions	"Advanced options"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance/{effectiveId}/advanced-options [get]
func getInstanceAdvancedOptions(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespace, effectiveId, err := utils.RetrieveNamespaceAndEid(w, r)
	if err != nil {
		return
	}

	opts, err := kovm.GetAdvancedOptions(r.Context(), namespace, effectiveId)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		log.Err(err).Msg("failed to get advanced options")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Error getting advanced options")
		return
	}

	b, _ := json.Marshal(opts)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// createInstance
//
//	@Summary		Create an Instance
//	@Description	Create an Instance
//	@Tags			v1, Instance
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Param			Body		body	kovm.CreateVMInfo	true	"Instance info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance [post]
func createInstance(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgId := chi.URLParam(r, "orgId")
	projectId := chi.URLParam(r, "projectId")
	namespace := utils.GetNamespace(projectId)

	err := k8s.CreateNamespaceIfNotExists(r.Context(), orgId, projectId)
	if err != nil {
		log.Err(err).Msg("failed to create namespace")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Error creating namespace")
		return
	}

	var body kovm.CreateVMInfo
	err = decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	for _, disk := range body.Disks {
		if disk.Eid != "" {
			// Check VM
			isMount, _, err := kovm.IsVMMountDisk(r.Context(), namespace, disk.Eid)
			if err != nil {
				log.Err(err).Str("namespace", namespace).Str("name", disk.Eid).Msg("Error checking vms")
				httpError.Http(w, r, http.StatusInternalServerError).Msg("Error checking vms")
				return
			}
			if isMount {
				log.Error().Err(err).Str("namespace", namespace).Str("name", disk.Eid).Msg("Cannot mount disk already used")
				httpError.Http(w, r, http.StatusBadRequest).Msg("Cannot mount disk already used")
				return
			}
		}

	}

	if err = kovm.CreateVM(r.Context(), namespace, body); err != nil {
		if strings.HasPrefix(err.Error(), "invalid container disk") {
			log.Err(err).Msg("Container disks rejected")
			httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
			return
		}
		if strings.HasPrefix(err.Error(), "invalid network ip") {
			log.Err(err).Msg("Network ip rejected")
			httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
			return
		}
		log.Err(err).Str("namespace", namespace).Msg("Error creating VM")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Error creating VM")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// updateInstance
//
//	@Summary		Update an Instance
//	@Description	Update an Instance
//	@Tags			v1, Instance
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Param			effectiveId	path	string				true	"Instance Effective ID"
//	@Param			Body		body	kovm.UpdateVMInfo	true	"Instance info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance/{effectiveId} [post]
func updateInstance(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgId := chi.URLParam(r, "orgId")
	projectId := chi.URLParam(r, "projectId")
	namespace := utils.GetNamespace(projectId)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	err := k8s.CreateNamespaceIfNotExists(r.Context(), orgId, projectId)
	if err != nil {
		log.Err(err).Msg("failed to create namespace")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Error creating namespace")
		return
	}

	var body kovm.UpdateVMInfo
	err = decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	for _, disk := range body.Disks {
		if disk.Eid != "" {
			// Check VM
			isMount, vmEid, err := kovm.IsVMMountDisk(r.Context(), namespace, disk.Eid)
			if err != nil {
				log.Err(err).Str("namespace", namespace).Str("name", effectiveId).Msg("Error check mount status")
				httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to check mount status")
				return
			}

			// If the disk is mounted on another VM than the one we update
			if isMount && vmEid != effectiveId {

				log.Info().Str("namespace", namespace).Str("name", effectiveId).Msg("Cannot mount disk already used")
				httpError.Http(w, r, http.StatusBadRequest).Msg("Cannot mount disk already used")
				return
			}
		}

	}

	if err = kovm.UpdateVM(r.Context(), namespace, effectiveId, body); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		if strings.HasPrefix(err.Error(), "invalid container disk") {
			log.Err(err).Msg("Container disks rejected")
			httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
			return
		}
		if strings.HasPrefix(err.Error(), "invalid network ip") {
			log.Err(err).Msg("Network ip rejected")
			httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
			return
		}
		log.Err(err).Str("namespace", namespace).Str("name", effectiveId).Msg("Error updating VM")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Error updating VM")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// deleteInstance
//
//	@Summary		Delete an Instance
//	@Description	Delete an Instance by Effective ID
//	@Tags			v1, Instance
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Instance Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance/{effectiveId} [delete]
func deleteInstance(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespace := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	if err := kovm.DeleteVM(r.Context(), namespace, effectiveId); err != nil {
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

// mountContainerDisk
//
//	@Summary		Mount a batch of container disks on an instance
//	@Description	Attach every resolved container disk in the batch to an existing VM in a single atomic Update.
//	@Tags			v1, ContainerDisk
//	@Produce		plain
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			projectId	path	string					true	"Project ID"
//	@Param			effectiveId	path	string					true	"Instance Effective ID"
//	@Param			Body		body	kovm.ContainerDiskBatch	true	"Batch of resolved container disk specs"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance/{effectiveId}/container-disk/mount [post]
func mountContainerDisk(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespace, effectiveId, err := utils.RetrieveNamespaceAndEid(w, r)
	if err != nil {
		return
	}

	var batch kovm.ContainerDiskBatch
	if err := decoder.HandleHTTPJSON(w, r, &batch, 5); err != nil {
		return
	}

	specs, err := kovm.ResolveByIDs(batch.Ids)
	if err != nil {
		log.Err(err).Msg("Container disk batch rejected")
		httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
		return
	}

	if err := kovm.MountContainerDisks(r.Context(), namespace, effectiveId, specs); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Container disk batch mount failed - VM not found")
			httpError.Http(w, r, http.StatusNotFound).Msg(http.StatusText(http.StatusNotFound))
			return
		}
		if strings.HasPrefix(err.Error(), "invalid container disk") {
			log.Err(err).Msg("Container disk batch rejected")
			httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
			return
		}
		log.Err(err).Str("namespace", namespace).Str("name", effectiveId).Int("count", len(specs)).Msg("Container disk batch mount failed")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Container disk batch mount failed")
		return
	}
	w.WriteHeader(http.StatusOK)
}

// unmountContainerDisk
//
//	@Summary		Unmount a batch of container disks from an instance
//	@Description	Detach every resolved container disk in the batch from an existing VM in a single atomic Update.
//	@Tags			v1, ContainerDisk
//	@Produce		plain
//	@Param			orgId		path	string					true	"Organization ID"
//	@Param			projectId	path	string					true	"Project ID"
//	@Param			effectiveId	path	string					true	"Instance Effective ID"
//	@Param			Body		body	kovm.ContainerDiskBatch	true	"Batch of resolved container disk specs"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance/{effectiveId}/container-disk/unmount [post]
func unmountContainerDisk(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespace, effectiveId, err := utils.RetrieveNamespaceAndEid(w, r)
	if err != nil {
		return
	}

	var batch kovm.ContainerDiskBatch
	if err := decoder.HandleHTTPJSON(w, r, &batch, 5); err != nil {
		return
	}

	specs, err := kovm.ResolveByIDs(batch.Ids)
	if err != nil {
		log.Err(err).Msg("Container disk batch rejected")
		httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
		return
	}

	if err := kovm.UnmountContainerDisks(r.Context(), namespace, effectiveId, specs); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Container disk batch unmount failed - VM not found")
			httpError.Http(w, r, http.StatusNotFound).Msg(http.StatusText(http.StatusNotFound))
			return
		}
		if strings.HasPrefix(err.Error(), "invalid container disk") {
			httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
			return
		}
		log.Err(err).Str("namespace", namespace).Str("name", effectiveId).Int("count", len(specs)).Msg("Container disk batch unmount failed")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Container disk batch unmount failed")
		return
	}
	w.WriteHeader(http.StatusOK)
}
