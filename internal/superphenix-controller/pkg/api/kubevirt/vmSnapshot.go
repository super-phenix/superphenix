package kubevirt

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubevirt/vmSnapshot"
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

const baseInstanceSnapshotEndpoint = "/instance-snapshot"

func InstanceSnapshotEndpoint(router chi.Router) {
	router.Route(baseInstanceSnapshotEndpoint, func(r chi.Router) {
		r.Get("/", listInstanceSnapshots)
		r.Post("/", createInstanceSnapshot)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getInstanceSnapshotByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getInstanceSnapshotByEffectiveId)
			r.Get("/restore", restoreInstanceSnapshot)
			r.Post("/clone", cloneInstanceSnapshot)

			r.Delete("/", deleteInstanceSnapshot)
		})
	})
}

// listInstanceSnapshots
//
//	@Summary		Retrieve all VmSnapshots
//	@Description	Retrieve all VmSnapshots for a project
//	@Tags			v1, VmSnapshot
//	@Produce		json
//	@Param			orgId		path	string			true	"Organization ID"
//	@Param			projectId	path	string			true	"Project ID"
//	@Success		200			{array}	view.VmSnapshot	"VmSnapshots"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance-snapshot [get]
func listInstanceSnapshots(w http.ResponseWriter, r *http.Request) {
	namespaceParam := utils.GetRequestNamespace(r)

	vmSnapshotList := vmSnapshot.ListVmSnapshots(r.Context(), namespaceParam)
	vmSnapshotContents := vmSnapshot.ListVmSnapshotContents(r.Context(), namespaceParam)

	var vmSnapshots []view.VmSnapshot
	for _, vmSnapshotItem := range vmSnapshotList {
		var vmSnapshotContentView view.VmSnapshotContentView
		if vmSnapshotItem.Status.VirtualMachineSnapshotContentName != nil {
			vmSnapshotContentView = vmSnapshotContents[*vmSnapshotItem.Status.VirtualMachineSnapshotContentName]
		}

		vmSnapshotR := view.VmSnapshotViewToResource(vmSnapshotItem, vmSnapshotContentView)
		vmSnapshots = append(vmSnapshots, vmSnapshotR)
	}

	b, _ := json.Marshal(vmSnapshots)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getInstanceSnapshotByLocalId
//
//	@Summary		Get VmSnapshot by local ID
//	@Description	Get VmSnapshot by local ID
//	@Tags			v1, VmSnapshot
//	@Produce		json
//	@Param			orgId		path		string			true	"Organization ID"
//	@Param			projectId	path		string			true	"Project ID"
//	@Param			localId		path		string			true	"VmSnapshot Local ID"
//	@Success		200			{object}	view.VmSnapshot	"VmSnapshot"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance-snapshot/localId/{localId} [get]
func getInstanceSnapshotByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Msg("failed to retrieve Resource Effective Id")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource effective Id")
		return
	}

	getVmSnapshot(w, r, effectiveId.(string))
}

// getInstanceSnapshotByEffectiveId
//
//	@Summary		Get VmSnapshot by effective ID
//	@Description	Get VmSnapshot by effective ID
//	@Tags			v1, VmSnapshot
//	@Produce		json
//	@Param			orgId		path		string			true	"Organization ID"
//	@Param			projectId	path		string			true	"Project ID"
//	@Param			effectiveId	path		string			true	"VmSnapshot Effective ID"
//	@Success		200			{object}	view.VmSnapshot	"VmSnapshot"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance-snapshot/{effectiveId} [get]
func getInstanceSnapshotByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")

	getVmSnapshot(w, r, effectiveId)
}

func getVmSnapshot(w http.ResponseWriter, r *http.Request, effectiveId string) {
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

	vmSnapshotView, err := vmSnapshot.GetVmSnapshot(r.Context(), namespace, effectiveId)
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

	contentName := ""
	if vmSnapshotView.Status.VirtualMachineSnapshotContentName != nil {
		contentName = *vmSnapshotView.Status.VirtualMachineSnapshotContentName
	}

	vmSnapshotContentView, err := vmSnapshot.GetVmSnapshotContent(r.Context(), namespace, contentName)
	if err != nil {
		log.Err(err).Msg("failed to retrieve VM Snapshot Content")
	}

	vmSnapshotResource := view.VmSnapshotViewToResource(vmSnapshotView, vmSnapshotContentView)
	b, _ := json.Marshal(vmSnapshotResource)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// createInstanceSnapshot
//
//	@Summary		Create a VmSnapshot
//	@Description	Create a VmSnapshot
//	@Tags			v1, VmSnapshot
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string							true	"Organization ID"
//	@Param			projectId	path	string							true	"Project ID"
//	@Param			Body		body	vmSnapshot.CreateVmSnapshotInfo	true	"VmSnapshot info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance-snapshot [post]
func createInstanceSnapshot(w http.ResponseWriter, r *http.Request) {
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

	var body vmSnapshot.CreateVmSnapshotInfo
	err = decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	if err = body.CreateVmSnapshot(r.Context(), namespace); err != nil {
		log.Error().Err(err).Msg("Failed to create instance snapshot")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create instance snapshot")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// deleteInstanceSnapshot
//
//	@Summary		Delete a VmSnapshot
//	@Description	Delete a VmSnapshot by Effective ID
//	@Tags			v1, VmSnapshot
//	@Produce		plain
//	@Param			orgId		path		string	true	"Organization ID"
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			effectiveId	path		string	true	"VmSnapshot Effective ID"
//	@Success		200			{string}	string	"deleted"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance-snapshot/{effectiveId} [delete]
func deleteInstanceSnapshot(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	if err := vmSnapshot.DeleteVmSnapshot(r.Context(), namespaceParam, effectiveId); err != nil {
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

// cloneInstanceSnapshot
//
//	@Summary		Clone a VmSnapshot
//	@Description	Clone a VmSnapshot by Effective ID
//	@Tags			v1, VmSnapshot
//	@Accept			json
//	@Produce		json
//	@Param			orgId		path		string							true	"Organization ID"
//	@Param			projectId	path		string							true	"Project ID"
//	@Param			effectiveId	path		string							true	"VmSnapshot Effective ID"
//	@Param			Body		body		vmSnapshot.CloneVmSnapshotInfo	true	"Clone info"
//	@Success		201			{object}	vmSnapshot.CloneVmSnapshotResponse
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance-snapshot/{effectiveId}/clone [post]
func cloneInstanceSnapshot(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgId, projectId, effectiveId := utils.GetRequestParams(r)
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	var body vmSnapshot.CloneVmSnapshotInfo
	if err := decoder.HandleHTTPJSON(w, r, &body, 5); err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	result, err := body.CloneVmSnapshot(r.Context(), orgId, projectId, effectiveId)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		log.Error().Err(err).Msg("Failed to clone instance snapshot")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to clone instance snapshot")
		return
	}

	b, _ := json.Marshal(result)
	ch.Data(w, http.StatusCreated, ch.MIMEJSON, b)
}

// restoreInstanceSnapshot
//
//	@Summary		Restore a VM Snapshot
//	@Description	Restore a VM Snapshot
//	@Tags			v1, VmSnapshot
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/instance-snapshot/{effectiveId}/restore [get]
func restoreInstanceSnapshot(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgId := chi.URLParam(r, "orgId")
	projectId := chi.URLParam(r, "projectId")

	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
	}

	if err := vmSnapshot.RestoreVmSnapshot(r.Context(), orgId, projectId, effectiveId); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}
		log.Err(err).Msg("Failed to restore vm snapshot")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to restore vm snapshot")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}
