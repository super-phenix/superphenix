package paas

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/dbaas"
	_ "github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	spxIdMiddleware "github.com/super-phenix/superphenix/pkg/superphenix-id/middleware"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	"github.com/go-chi/chi/v5"
	"k8s.io/apimachinery/pkg/api/errors"
)

const baseDBaaSEndpoint = "/dbaas"

func DBaaSEndpoint(router chi.Router) {
	router.Route(baseDBaaSEndpoint, func(r chi.Router) {
		r.Get("/", listDBaaS)

		r.With(spxIdMiddleware.AddEffectiveIdToContext()).Get("/localId/{localId}", getDBaaSByLocalId)
		r.Route("/{effectiveId}", func(r chi.Router) {
			r.Get("/", getDBaaSByEffectiveId)
			r.Get("/credentials", getDBaaSCredentials)
			r.Post("/restart", restartDBaaS)
		})
	})
}

// listDBaaS
//
//	@Summary		Retrieve all dbaas
//	@Description	Retrieve all CloudNative-PG clusters for a project
//	@Tags			v1, DBaaS
//	@Produce		json
//	@Param			orgId		path	string		true	"Organization ID"
//	@Param			projectId	path	string		true	"Project ID"
//	@Success		200			{array}	view.DBaaS	"DBaaS List"
//	@Failure		500
//	@Router			/{orgId}/{projectId}/dbaas [get]
func listDBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())

	databases, err := dbaas.ListDBaaS(r.Context(), utils.GetRequestNamespace(r))
	if err != nil {
		log.Err(err).Msg("failed to retrieve resource")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve resource")
		return
	}

	b, _ := json.Marshal(databases)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getDBaaSByLocalId
//
//	@Summary		Get dbaas by local ID
//	@Description	Get a CloudNative-PG cluster by local ID
//	@Tags			v1, DBaaS
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			localId		path		string		true	"DBaaS Local ID"
//	@Success		200			{object}	view.DBaaS	"DBaaS"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/dbaas/localId/{localId} [get]
func getDBaaSByLocalId(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	effectiveId, ok := r.Context().Value(spxId.EffectiveIdContext()).(string)
	if !ok || effectiveId == "" {
		log.Error().Msg("failed to retrieve Resource Effective Id")
		httpError.Http(w, r, http.StatusBadRequest).Msg("failed to retrieve Resource Effective Id")
		return
	}
	getDBaaS(w, r, effectiveId)
}

// getDBaaSByEffectiveId
//
//	@Summary		Get dbaas by effective ID
//	@Description	Get a CloudNative-PG cluster by effective ID
//	@Tags			v1, DBaaS
//	@Produce		json
//	@Param			orgId		path		string		true	"Organization ID"
//	@Param			projectId	path		string		true	"Project ID"
//	@Param			effectiveId	path		string		true	"DBaaS Effective ID"
//	@Success		200			{object}	view.DBaaS	"DBaaS"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/dbaas/{effectiveId} [get]
func getDBaaSByEffectiveId(w http.ResponseWriter, r *http.Request) {
	_, effectiveId, err := utils.RetrieveNamespaceAndEid(w, r)
	if err != nil {
		return
	}
	getDBaaS(w, r, effectiveId)
}

func getDBaaS(w http.ResponseWriter, r *http.Request, effectiveId string) {
	log := logger.GetLogger(r.Context())

	database, err := dbaas.GetDBaaS(r.Context(), utils.GetRequestNamespace(r), effectiveId)
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

	b, _ := json.Marshal(database)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// getDBaaSCredentials
//
//	@Summary		Get dbaas credentials
//	@Description	Get the application user credentials and the load balancer endpoints of a CloudNative-PG cluster
//	@Tags			v1, DBaaS
//	@Produce		json
//	@Param			orgId		path		string				true	"Organization ID"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			effectiveId	path		string				true	"DBaaS Effective ID"
//	@Success		200			{object}	dbaas.Credentials	"Credentials"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/dbaas/{effectiveId}/credentials [get]
func getDBaaSCredentials(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespace, effectiveId, err := utils.RetrieveNamespaceAndEid(w, r)
	if err != nil {
		return
	}

	credentials, err := dbaas.GetCredentials(r.Context(), namespace, effectiveId)
	if errors.IsNotFound(err) {
		log.Err(err).Msg("Resource not found")
		httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
		return
	}
	if err != nil {
		log.Err(err).Msg("failed to retrieve credentials")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to retrieve credentials")
		return
	}

	b, _ := json.Marshal(credentials)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// restartDBaaS
//
//	@Summary		Restart a dbaas
//	@Description	Rolling restart of the instances of a CloudNative-PG cluster
//	@Tags			v1, DBaaS
//	@Param			orgId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"DBaaS Effective ID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgId}/{projectId}/dbaas/{effectiveId}/restart [post]
func restartDBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	namespace, effectiveId, err := utils.RetrieveNamespaceAndEid(w, r)
	if err != nil {
		return
	}

	err = dbaas.Restart(r.Context(), namespace, effectiveId)
	if errors.IsNotFound(err) {
		log.Err(err).Msg("Resource not found")
		httpError.Http(w, r, http.StatusNotFound).Msg("Resource not found")
		return
	}
	if err != nil {
		log.Err(err).Msg("failed to restart resource")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("failed to restart resource")
		return
	}

	w.WriteHeader(http.StatusOK)
}
