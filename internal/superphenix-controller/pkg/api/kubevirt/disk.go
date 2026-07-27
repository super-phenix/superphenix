package kubevirt

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s/pvc"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubevirt/datavolume"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubevirt/vm"
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

const baseDiskEndpoint = "/disk"

func DiskEndpoint(router chi.Router) {
	router.Route(baseDiskEndpoint, func(r chi.Router) {
		r.Get("/", listDisks)
		r.Post("/", createDisk)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getDiskByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getDiskByEffectiveId)

			r.Post("/", updateDisk)
			r.Get("/unmount", unmountDisk)
			r.Delete("/", deleteDisk)
		})
	})
}

// listDisks
//
//	@Summary		Retrieve all Disks
//	@Description	Retrieve all Disks for a project
//	@Tags			v1, Disk
//	@Produce		json
//	@Param			orgId		path	string		true	"Organization ID"
//	@Param			projectId	path	string		true	"Project ID"
//	@Success		200			{array}	view.Disk	"Disks"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/disk [get]
func listDisks(w http.ResponseWriter, r *http.Request) {
	namespaceParam := utils.GetRequestNamespace(r)

	diskList := datavolume.ListDisks(r.Context(), namespaceParam)
	pvcList := pvc.ListPVCs(r.Context(), namespaceParam)
	vmList := vm.ListVM(r.Context(), namespaceParam)

	var disks []view.Disk
	for _, pvcItem := range pvcList {
		// We don't want prime pvc as they are temp pvc
		if strings.HasPrefix(pvcItem.Name, "prime-") {
			continue
		}
		var disk view.DiskView
		for _, diskItem := range diskList {
			if diskItem.Name == pvcItem.Name {
				disk = diskItem
				break
			}
		}

		// Check VM
		isMount, vmName := vm.IsVMListMountDisk(vmList, pvcItem.Name)
		diskR := view.DiskViewToResource(disk, pvcItem)
		if isMount {
			diskR.MountStatus.IsMounted = isMount
			diskR.MountStatus.By = vmName
		}
		disks = append(disks, diskR)
	}

	b, _ := json.Marshal(disks)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getDiskByLocalId
//
//	@Summary		Get Disk by local ID
//	@Description	Get Disk by local ID
//	@Tags			v1, Disk
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			localId		path		string		true	"Disk Local ID"
//	@Success		200			{object}	view.Disk	"Disk"
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/disk/localId/{localId} [get]
func getDiskByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Msg("failed to retrieve Resource Effective Id")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource effective Id")
		return
	}

	getDisk(w, r, effectiveId.(string))
}

// getDiskByEffectiveId
//
//	@Summary		Get Disk by effective ID
//	@Description	Get Disk by effective ID
//	@Tags			v1, Disk
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			effectiveId	path		string		true	"Disk Effective ID"
//	@Success		200			{object}	view.Disk	"Disk"
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/disk/{effectiveId} [get]
func getDiskByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")

	getDisk(w, r, effectiveId)
}

func getDisk(w http.ResponseWriter, r *http.Request, effectiveId string) {
	log := logger.GetLogger(r.Context())
	namespace := utils.GetRequestNamespace(r)
	if namespace == "" {
		log.Error().Msg("no namespace provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no project Id provided")
		return
	}

	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	pvcView, err := pvc.GetPVC(r.Context(), namespace, effectiveId)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("eid", effectiveId).Msg("Error getting PVC")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}

	diskView, err := datavolume.GetDisk(r.Context(), namespace, effectiveId)
	if errors.IsNotFound(err) && pvcView.Name == "" {
		log.Err(err).Str("namespace", namespace).Str("eid", effectiveId).Msg("PVC and disk not found")
		httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
		return
	}
	if err != nil && !errors.IsNotFound(err) {
		log.Err(err).Str("namespace", namespace).Str("eid", effectiveId).Msg("Error getting disk")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}

	// Check VM
	isMount, vmName, err := vm.IsVMMountDisk(r.Context(), namespace, pvcView.Name)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", pvcView.Name).Msg("Error check mount status")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to check mount status")
		return
	}

	diskResource := view.DiskViewToResource(diskView, pvcView)

	if isMount {
		diskResource.MountStatus.IsMounted = isMount
		diskResource.MountStatus.By = vmName
	}

	b, _ := json.Marshal(diskResource)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// createDisk
//
//	@Summary		Create a Disk
//	@Description	Create a Disk
//	@Tags			v1, Disk
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string						true	"Organization ID"
//	@Param			projectId	path	string						true	"Project ID"
//	@Param			Body		body	datavolume.CreateDiskInfo	true	"Disk info"
//	@Success		200
//	@Failure		400
//	@Router			/{orgId}/{projectId}/disk [post]
func createDisk(w http.ResponseWriter, r *http.Request) {
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

	var body datavolume.CreateDiskInfo
	err = decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	err = body.CreateDisk(r.Context(), namespace)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create disk")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create disk")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// updateDisk
//
//	@Summary		Update a Disk
//	@Description	Update a Disk
//	@Tags			v1, Disk
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Param			effectiveId	path	string				true	"Instance Effective ID"
//	@Param			Body		body	pvc.UpdateDiskInfo	true	"Disk info"
//	@Success		200
//	@Failure		400
//	@Router			/{orgId}/{projectId}/disk/{effectiveId} [post]
func updateDisk(w http.ResponseWriter, r *http.Request) {
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
		log.Error().Msg("Failed to create namespace")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create namespace")
		return
	}

	var body pvc.UpdateDiskInfo
	err = decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	if err = body.UpdatePVC(r.Context(), namespace, effectiveId); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		log.Error().Err(err).Msg("Failed to update disk")
		httpError.Http(w, r, http.StatusInternalServerError).Msg(err.Error())
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// unmountDisk
//
//	@Summary		Unmount a Disk
//	@Description	Unmount a Disk by Effective ID
//	@Tags			v1, Disk
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Disk Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Router			/{orgId}/{projectId}/disk/{effectiveId}/unmount [get]
func unmountDisk(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)

	if namespaceParam == "" {
		log.Error().Msg("no namespace provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no project Id provided")
		return
	}

	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	// Check VM
	isMount, vmEid, err := vm.IsVMMountDisk(r.Context(), namespaceParam, effectiveId)
	if err != nil {
		log.Err(err).Str("namespace", namespaceParam).Str("name", effectiveId).Msg("Error check mount status")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to check mount status")
	}
	if !isMount {
		log.Info().Str("namespace", namespaceParam).Str("name", effectiveId).Msg("Disk not mounted")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Disk not mounted")
		return
	}

	if err := vm.UnmountDisk(r.Context(), namespaceParam, vmEid, effectiveId); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Str("namespace", namespaceParam).Str("name", effectiveId).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg(http.StatusText(http.StatusNotFound))
		} else {
			log.Err(err).Str("namespace", namespaceParam).Str("name", effectiveId).Msg("Failed to unmount")
			httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to unmount")
		}
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// deleteDisk
//
//	@Summary		Delete a Disk
//	@Description	Delete a Disk by Effective ID
//	@Tags			v1, Disk
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Disk Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Router			/{orgId}/{projectId}/disk/{effectiveId} [delete]
func deleteDisk(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	// Check VM
	isMount, _, err := vm.IsVMMountDisk(r.Context(), namespaceParam, effectiveId)
	if err != nil {
		log.Err(err).Str("namespace", namespaceParam).Str("name", effectiveId).Msg("Error check mount status")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to check mount status")
	}
	if isMount {
		log.Info().Str("namespace", namespaceParam).Str("name", effectiveId).Msg("Cannot delete mounted disk")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Cannot delete mounted disk")
		return
	}

	errDisk := datavolume.DeleteDisk(r.Context(), namespaceParam, effectiveId)
	if errDisk != nil && !errors.IsNotFound(errDisk) {
		log.Err(errDisk).Msg("Error deleting Datavolume")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to delete disk")
		return
	}

	errPVC := pvc.DeletePVC(r.Context(), namespaceParam, effectiveId)
	if errPVC != nil && !errors.IsNotFound(errPVC) {
		log.Err(errDisk).Msg("Error deleting PVC")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to delete disk")
		return
	}

	if errors.IsNotFound(errDisk) && errors.IsNotFound(errPVC) {
		log.Err(err).Msg("Failed to delete resource - Not found")
		httpError.Http(w, r, http.StatusNotFound).Msg(http.StatusText(http.StatusNotFound))
		return
	}

	w.WriteHeader(http.StatusOK)
}
