package k8s

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s"
	_ "github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/objectbucket"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	spxIdMiddleware "github.com/super-phenix/superphenix/pkg/superphenix-id/middleware"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	"github.com/go-chi/chi/v5"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const baseBucketEndpoint = "/bucket"

func BucketEndpoint(router chi.Router) {
	router.Route(baseBucketEndpoint, func(r chi.Router) {
		r.Get("/", listBuckets)
		r.Post("/", createBucket)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getBucketByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getBucketByEffectiveId)

			r.Post("/", updateBucket)
			r.Delete("/", deleteBucket)
			r.Get("/credentials", getBucketCredentials)
		})
	})
}

// listBuckets
//
//	@Summary		Retrieve all Buckets
//	@Description	Retrieve all S3 Buckets for a project
//	@Tags			v1, Bucket
//	@Produce		json
//	@Param			orgId		path	string		true	"Organization ID"
//	@Param			projectId	path	string		true	"Project ID"
//	@Success		200			{array}	view.Bucket	"Buckets"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/bucket [get]
func listBuckets(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespace := utils.GetRequestNamespace(r)

	buckets, err := objectbucket.ListBuckets(r.Context(), namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error listing buckets")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}

	b, _ := json.Marshal(buckets)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getBucketByLocalId
//
//	@Summary		Get Bucket by local ID
//	@Description	Get Bucket by local ID
//	@Tags			v1, Bucket
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			localId		path		string		true	"Bucket Local ID"
//	@Success		200			{object}	view.Bucket	"Bucket"
//	@Failure		400
//	@Failure		500
//	@Router			/{orgId}/{projectId}/bucket/localId/{localId} [get]
func getBucketByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId := r.Context().Value(spxId.EffectiveIdContext())
	if effectiveId == nil {
		log.Error().Msg("failed to retrieve Resource Effective Id")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource effective Id")
		return
	}

	getBucket(w, r, effectiveId.(string))
}

// getBucketByEffectiveId
//
//	@Summary		Get Bucket by effective ID
//	@Description	Get Bucket by effective ID
//	@Tags			v1, Bucket
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			effectiveId	path		string		true	"Bucket Effective ID"
//	@Success		200			{object}	view.Bucket	"Bucket"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/bucket/{effectiveId} [get]
func getBucketByEffectiveId(w http.ResponseWriter, r *http.Request) {
	effectiveId := chi.URLParam(r, "effectiveId")

	getBucket(w, r, effectiveId)
}

func getBucket(w http.ResponseWriter, r *http.Request, effectiveId string) {
	log := logger.GetLogger(r.Context())
	namespace := utils.GetRequestNamespace(r)

	if effectiveId == "" {
		log.Error().Msg("no Resource Effective Id provided")
		httpError.Http(w, r, http.StatusBadRequest).Msg("no Resource Effective Id provided")
		return
	}

	bucket, err := objectbucket.GetBucket(r.Context(), namespace, effectiveId)
	if apierrors.IsNotFound(err) {
		log.Err(err).Str("namespace", namespace).Str("eid", effectiveId).Msg("Bucket not found")
		httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
		return
	}
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("eid", effectiveId).Msg("Error getting bucket")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}

	b, _ := json.Marshal(bucket)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// createBucket
//
//	@Summary		Create a Bucket
//	@Description	Create an S3 Bucket
//	@Tags			v1, Bucket
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string							true	"Organization ID"
//	@Param			projectId	path	string							true	"Project ID"
//	@Param			Body		body	objectbucket.CreateBucketInfo	true	"Bucket info"
//	@Success		200
//	@Failure		400
//	@Failure		409
//	@Router			/{orgId}/{projectId}/bucket [post]
func createBucket(w http.ResponseWriter, r *http.Request) {
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

	var body objectbucket.CreateBucketInfo
	err = decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	err = body.CreateBucket(r.Context(), namespace)
	var validationErr objectbucket.ValidationError
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case errors.As(err, &validationErr):
		log.Error().Err(err).Msg("Invalid bucket request")
		httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
	case apierrors.IsAlreadyExists(err):
		log.Error().Err(err).Msg("Bucket already exists")
		httpError.Http(w, r, http.StatusConflict).Msg("Bucket already exists")
	default:
		log.Error().Err(err).Msg("Failed to create bucket")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to create bucket")
	}
}

// updateBucket
//
//	@Summary		Update a Bucket
//	@Description	Update an S3 Bucket configuration (quotas, policy, lifecycle)
//	@Tags			v1, Bucket
//	@Accept			json
//	@Produce		plain
//	@Param			orgId		path	string							true	"Organization ID"
//	@Param			projectId	path	string							true	"Project ID"
//	@Param			effectiveId	path	string							true	"Bucket Effective ID"
//	@Param			Body		body	objectbucket.UpdateBucketInfo	true	"Bucket info"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Router			/{orgId}/{projectId}/bucket/{effectiveId} [post]
func updateBucket(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespace, effectiveId, err := utils.RetrieveNamespaceAndEid(w, r)
	if err != nil {
		return
	}

	var body objectbucket.UpdateBucketInfo
	err = decoder.HandleHTTPJSON(w, r, &body, 5)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode body")
		httpError.Http(w, r, http.StatusBadRequest).Msg("Failed to decode body")
		return
	}

	err = body.UpdateBucket(r.Context(), namespace, effectiveId)
	var validationErr objectbucket.ValidationError
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case errors.As(err, &validationErr):
		log.Error().Err(err).Msg("Invalid bucket request")
		httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
	case apierrors.IsNotFound(err):
		log.Err(err).Msg("Resource not found")
		httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
	default:
		log.Error().Err(err).Msg("Failed to update bucket")
		httpError.Http(w, r, http.StatusInternalServerError).Msg(err.Error())
	}
}

// deleteBucket
//
//	@Summary		Delete a Bucket
//	@Description	Delete an S3 Bucket by Effective ID
//	@Tags			v1, Bucket
//	@Produce		plain
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Bucket Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Router			/{orgId}/{projectId}/bucket/{effectiveId} [delete]
func deleteBucket(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespace, effectiveId, err := utils.RetrieveNamespaceAndEid(w, r)
	if err != nil {
		return
	}

	err = objectbucket.DeleteBucket(r.Context(), namespace, effectiveId)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case apierrors.IsNotFound(err):
		log.Err(err).Msg("Failed to delete resource - Not found")
		httpError.Http(w, r, http.StatusNotFound).Msg(http.StatusText(http.StatusNotFound))
	default:
		log.Err(err).Msg("Error deleting bucket")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to delete bucket")
	}
}

// getBucketCredentials
//
//	@Summary		Get Bucket credentials
//	@Description	Get the S3 endpoint and admin credentials generated for a Bucket
//	@Tags			v1, Bucket
//	@Produce		json
//	@Param			orgId		path		string						true	"Organization ID"
//	@Param			projectId	path		string						true	"Project ID"
//	@Param			effectiveId	path		string						true	"Bucket Effective ID"
//	@Success		200			{object}	objectbucket.Credentials	"Credentials"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/bucket/{effectiveId}/credentials [get]
func getBucketCredentials(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespace, effectiveId, err := utils.RetrieveNamespaceAndEid(w, r)
	if err != nil {
		return
	}

	credentials, err := objectbucket.GetCredentials(r.Context(), namespace, effectiveId)
	if apierrors.IsNotFound(err) {
		log.Err(err).Str("namespace", namespace).Str("eid", effectiveId).Msg("Bucket credentials not found")
		httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
		return
	}
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("eid", effectiveId).Msg("Error getting bucket credentials")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve credentials")
		return
	}

	b, _ := json.Marshal(credentials)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}
