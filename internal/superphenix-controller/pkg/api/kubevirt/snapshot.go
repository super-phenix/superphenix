package kubevirt

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubevirt/volumeSnapshot"
	_ "github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"
	spxIdMiddleware "github.com/super-phenix/superphenix/pkg/superphenix-id/middleware"

	"github.com/go-chi/chi/v5"
	"k8s.io/apimachinery/pkg/api/errors"
)

const baseSnapshotEndpoint = "/snapshot"

func SnapshotEndpoint(router chi.Router) {
	router.Route(baseSnapshotEndpoint, func(r chi.Router) {
		r.Get("/", listSnapshots)
		r.Post("/", createSnapshot)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getSnapshotByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getSnapshotByEffectiveId)
			r.Post("/", updateSnapshot)
			r.Delete("/", deleteSnapshot)
		})
	})
}

// listSnapshots
//
//	@Summary		Retrieve all Snapshots
//	@Description	Retrieve all Snapshots for a project
//	@Tags			v1, Snapshot
//	@Produce		json
//	@Param			orgId		path	string			true	"Organization ID"
//	@Param			projectId	path	string			true	"Project ID"
//	@Success		200			{array}	view.Snapshot	"Snapshots"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/snapshot [get]
func listSnapshots(w http.ResponseWriter, r *http.Request) {
	namespaceParam := utils.GetRequestNamespace(r)

	snapshots := volumeSnapshot.ListSnapshots(r.Context(), namespaceParam)

	b, _ := json.Marshal(snapshots)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getSnapshotByLocalId
//
//	@Summary		Get Snapshot by local ID
//	@Description	Get Snapshot by local ID
//	@Tags			v1, Snapshot
//	@Produce		json
//	@Param			orgId		path		string			true	"Organization ID"
//	@Param			projectId	path		string			true	"Project ID"
//	@Param			localId		path		string			true	"Snapshot Local ID"
//	@Success		200			{object}	view.Snapshot	"Snapshot"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/snapshot/localId/{localId} [get]
func getSnapshotByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Msg("failed to retrieve Resource Effective Id")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource effective Id")
		return
	}

	getSnapshot(w, r, effectiveId.(string))
}

// getSnapshotByEffectiveId
//
//	@Summary		Get Snapshot by effective ID
//	@Description	Get Snapshot by effective ID
//	@Tags			v1, Snapshot
//	@Produce		json
//	@Param			orgId		path		string			true	"Organization ID"
//	@Param			projectId	path		string			true	"Project ID"
//	@Param			effectiveId	path		string			true	"Snapshot Effective ID"
//	@Success		200			{object}	view.Snapshot	"Snapshot"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/snapshot/{effectiveId} [get]
func getSnapshotByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")

	getSnapshot(w, r, effectiveId)
}

func getSnapshot(w http.ResponseWriter, r *http.Request, effectiveId string) {
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

	snapshotResource, err := volumeSnapshot.GetSnapshot(r.Context(), namespace, effectiveId)
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

	b, _ := json.Marshal(snapshotResource)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// createSnapshot
//
//	@Summary		Create a Snapshot
//	@Description	Create a Snapshot
//	@Tags			v1, Snapshot
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string								true	"Organization ID"
//	@Param			projectId	path	string								true	"Project ID"
//	@Param			Body		body	volumeSnapshot.CreateSnapshotInfo	true	"Snapshot info"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/snapshot [post]
func createSnapshot(w http.ResponseWriter, r *http.Request) {
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

	var body volumeSnapshot.CreateSnapshotInfo
	err = decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	if err = body.CreateSnapshot(r.Context(), namespace); err != nil {
		log.Error().Err(err).Msg("Failed to create snapshot")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create snapshot")
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// updateSnapshot
//
//	@Summary		Update a Snapshot Schedule
//	@Description	Update a Snapshot Schedule by Effective ID
//	@Tags			v1, Snapshot
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string								true	"Organization ID"
//	@Param			projectId	path	string								true	"Project ID"
//	@Param			effectiveId	path	string								true	"Snapshot Effective ID"
//	@Param			Body		body	volumeSnapshot.UpdateSnapshotInfo	true	"Snapshot Schedule info"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/snapshot/{effectiveId} [post]
func updateSnapshot(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	var body volumeSnapshot.UpdateSnapshotInfo
	err := decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	if err = body.UpdateSnapshot(r.Context(), namespaceParam, effectiveId); err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Failed to update snapshot - Not found")
			httpError.Http(w, r, http.StatusNotFound).Msg(http.StatusText(http.StatusNotFound))
		} else {
			log.Error().Err(err).Msg("Failed to update snapshot")
			httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to update snapshot")
		}
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

// deleteSnapshot
//
//	@Summary		Delete a Snapshot
//	@Description	Delete a Snapshot by Effective ID
//	@Tags			v1, Snapshot
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Snapshot Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/snapshot/{effectiveId} [delete]
func deleteSnapshot(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespaceParam := utils.GetRequestNamespace(r)
	effectiveId := chi.URLParam(r, "effectiveId")
	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	// Check if the snapshot is not owned by a VM Snapshot
	snapshotResource, err := volumeSnapshot.GetSnapshot(r.Context(), namespaceParam, effectiveId)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Err(err).Msg("Resource not found")
			httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
			return
		}

		log.Err(err).Msg("Failed to delete snapshot")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to delete snapshot")
		return
	}

	if snapshotResource.Snapshot != nil && len(snapshotResource.Snapshot.OwnerReferences) > 0 {
		ownedByScheduleOnly := true
		for _, ref := range snapshotResource.Snapshot.OwnerReferences {
			if ref.APIVersion != "snapscheduler.backube/v1" || ref.Kind != "SnapshotSchedule" {
				ownedByScheduleOnly = false
				break
			}
		}
		if !ownedByScheduleOnly {
			log.Err(fmt.Errorf("cannot delete snapshot - linked to a VM Snapshot")).Msg("Failed to delete Snapshot")
			httpError.Http(w, r, http.StatusBadRequest).Msg("cannot delete snapshot linked to a instance Snapshot")
			return
		}
	}

	if err := volumeSnapshot.DeleteSnapshot(r.Context(), namespaceParam, effectiveId); err != nil {
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
