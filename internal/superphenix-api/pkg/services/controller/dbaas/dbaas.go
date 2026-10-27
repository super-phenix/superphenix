package dbaas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/view"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/az"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/product"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/proxy"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"
	argodbaas "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app/dbaas"
	ctrlutils "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/utils"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// ListDBaaS
//
//	@Summary		Retrieve all DBaaS
//	@Description	Retrieve all PostgreSQL databases of a project, with their status
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Success		200			{array}	DBaaSFullResponse	"DBaaS"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{projectId}/dbaas [get]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSRead]
func (h *Service) ListDBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgDb, projectDb, code, errMsg := ctrlutils.CheckListPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	urls := az.FindAll(orgDb.ID.String())

	responses, err := proxy.SendBatchProxy(r, urls, config.ApiPrefix)
	if err != nil {
		log.Err(err).Msg(consts.SpxProxyToAZFailure)
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Msg(consts.SpxProxyToAZFailure)
		return
	}
	concatResults := ctrlutils.ConcatResponses(r.Context(), responses)

	resourcesDb, err := product.FindAllByProjectIdAndResourceType(projectDb.ID.String(), model.ProductTypeDBaaS.Name)
	if err != nil {
		log.Err(err).
			Str("projectId", projectDb.ID.String()).
			Str("resourceType", model.ProductTypeDBaaS.Name).
			Msg(consts.SpxFindAllResourcesError)
		httpError.Http(w, r, consts.SpxFindAllResourcesErrorCode).Msg(consts.SpxFindAllResourcesError)
		return
	}

	// Keep only resources from available azs
	resources := make([]model.Product, 0, len(resourcesDb))
	for _, p := range resourcesDb {
		if slices.ContainsFunc(urls, func(u config.AZConfig) bool { return u.Code == p.CodeAZ }) {
			resources = append(resources, p)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(combineListResult(concatResults, resources))
	if err != nil {
		log.Err(err).Msg(consts.SpxResponseParseFailure)
		httpError.Http(w, r, consts.SpxResponseParseFailureCode).Msg(consts.SpxResponseParseFailure)
		return
	}
	_, _ = w.Write(marshal)
}

// GetPostgresVersions
//
//	@Summary		Get supported PostgreSQL versions
//	@Description	Get supported PostgreSQL versions
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Success		200			{array}	string	"PostgreSQL Versions"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{projectId}/dbaas/postgres-versions [get]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSRead]
func (h *Service) GetPostgresVersions(w http.ResponseWriter, r *http.Request) {
	postgresVersions := h.cfg.ProductsConfig.ArgoApp.DBaaS.PostgresVersions
	versions := make([]string, 0, len(postgresVersions))
	for _, v := range postgresVersions {
		versions = append(versions, v.Version)
	}
	b, _ := json.Marshal(versions)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// GetDBaaS
//
//	@Summary		Get DBaaS instance
//	@Description	Get a PostgreSQL database by effective ID, with its status
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path		string				true	"Organization ID"
//	@Param			az			path		string				true	"AZ Code"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			effectiveId	path		string				true	"DBaaS EID"
//	@Success		200			{object}	DBaaSFullResponse	"DBaaS"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/dbaas/{effectiveId} [get]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSRead]
func (h *Service) GetDBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, _, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	if !ctrlutils.CheckProductBelongsToProject(w, r, projectDb.ID, azDb.Code) {
		return
	}

	resp, err := proxy.SendProxy(r, azDb, config.ApiPrefix, http.NoBody)
	if err != nil {
		log.Error().Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
	} else {
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			log.Error().Str("path", r.URL.Path).Str("status", resp.Status).Int("statusCode", resp.StatusCode).Str("az", azDb.Code).Msg("Request on superphenix-controller failed")
		}
	}

	resourceEId := chi.URLParam(r, "effectiveId")
	dbProduct, dbErr := product.FindByEId(resourceEId)

	productResponse, azResult, outcome := controller.ResolveProductResponse(r.Context(), azDb.Code, resp, dbProduct, dbErr)
	if outcome == controller.MergeNotFound {
		log.Error().Str("effectiveId", resourceEId).Msg(consts.SpxResourceNotFound)
		httpError.Http(w, r, http.StatusNotFound).Str("eid", resourceEId).Msg(consts.SpxResourceNotFound)
		return
	}

	result := DBaaSFullResponse{ProductResponse: productResponse}
	if azResult != nil {
		result.Cluster = azResult["cluster"]
		result.Endpoints = azResult["endpoints"]
	}

	// External access is only known by the Application, a database without one is reported without it.
	app, appFound, err := h.fetchDBaaSApp(r.Context(), projectDb.ID.String(), resourceEId)
	if err != nil {
		log.Err(err).Str("effectiveId", resourceEId).Msg("Failed to fetch app")
	}
	if appFound && app.Spec.ExternalAccess {
		result.ExternalAccess = externalAccess(r.Context(), projectDb.ID.String(), productResponse.ID)
	}

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(result)
	if err != nil {
		log.Err(err).Msg(consts.SpxResponseParseFailure)
		httpError.Http(w, r, consts.SpxResponseParseFailureCode).Msg(consts.SpxResponseParseFailure)
		return
	}
	_, _ = w.Write(marshal)
}

// GetForUpdateDBaaS
//
//	@Summary		Get DBaaS App
//	@Description	Get the DBaaS spec for the update form
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path		string					true	"Organization ID"
//	@Param			az			path		string					true	"AZ Code"
//	@Param			projectId	path		string					true	"Project ID"
//	@Param			effectiveId	path		string					true	"DBaaS EID"
//	@Success		200			{object}	DBaaSAppSpecResponse	"DBaaS"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/dbaas/{effectiveId}/app [get]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSRead]
func (h *Service) GetForUpdateDBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, _, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	if !ctrlutils.CheckProductBelongsToProject(w, r, projectDb.ID, azDb.Code) {
		return
	}

	resourceEId := chi.URLParam(r, "effectiveId")
	dbProduct, dbErr := product.FindByEId(resourceEId)

	app, appFound, err := h.fetchDBaaSApp(r.Context(), projectDb.ID.String(), resourceEId)
	if err != nil {
		log.Err(err).Msg("Failed to fetch app")
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("eid", resourceEId).Msg(consts.SpxProxyToAZFailure)
		return
	}

	if !appFound || dbErr != nil {
		log.Error().Str("effectiveId", resourceEId).Msg(consts.SpxResourceNotFound)
		httpError.Http(w, r, http.StatusNotFound).Str("eid", resourceEId).Msg(consts.SpxResourceNotFound)
		return
	}

	result := DBaaSAppSpecResponse{
		ProductResponse: ProductResponse{
			ID:            dbProduct.ID.String(),
			EId:           dbProduct.EffectiveID,
			ProductName:   dbProduct.ProductName,
			CodeAZ:        azDb.Code,
			ProductTypeId: dbProduct.ProductTypeId,
			Gitops:        app.Gitops,
		},
		Spec: app.Spec,
	}

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(result)
	if err != nil {
		log.Err(err).Msg(consts.SpxResponseParseFailure)
		httpError.Http(w, r, consts.SpxResponseParseFailureCode).Msg(consts.SpxResponseParseFailure)
		return
	}
	_, _ = w.Write(marshal)
}

// CreateDBaaS
//
//	@Summary		Create DBaaS
//	@Description	Create a PostgreSQL database, and its EIP when external access is enabled
//	@Tags			v1, SPX Argo Ctrl
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path		string			true	"Organization ID"
//	@Param			az			path		string			true	"AZ Code"
//	@Param			projectId	path		string			true	"Project ID"
//	@Param			Body		body		CreateDBaaSBody	true	"DBaaS info"
//	@Success		200			{object}	controller.CreateResponse
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/dbaas [post]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSWrite]
func (h *Service) CreateDBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, orgDb, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	var body CreateDBaaSBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	chart, supported := argodbaas.ChartFromConfig(body.Spec.Version)
	if !supported {
		log.Error().Str("version", body.Spec.Version).Msg("PostgreSQL version not supported")
		httpError.Http(w, r, http.StatusBadRequest).Msg("PostgreSQL version not supported")
		return
	}

	dbaasConfig, err := getDBaaSConfig(r.Context(), azDb, orgDb.ID.String(), projectDb.ID.String())
	if err != nil {
		log.Err(err).Msg("Failed to get dbaas configuration from AZ")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	subnet, err := resolveSubnet(r.Context(), azDb, orgDb.ID.String(), projectDb.ID.String(), body.Spec.SubnetEId)
	if err != nil {
		log.Err(err).Str("subnet", body.Spec.SubnetEId).Msg("Failed to resolve subnet")
		httpError.Http(w, r, http.StatusBadRequest).Msg("subnet not found")
		return
	}

	vipFree, err := ctrlutils.VIPAvailable(r.Context(), azDb, orgDb.ID.String(), projectDb.ID.String(), body.Spec.VIP, body.Spec.SubnetEId)
	var statusErr *ctrlutils.AZStatusError
	if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusBadRequest {
		httpError.Http(w, r, http.StatusBadRequest).Msg(statusErr.Message)
		return
	}
	if err != nil {
		log.Err(err).Msg("Failed to check the VIP availability")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}
	if !vipFree {
		httpError.Http(w, r, http.StatusBadRequest).Msgf("vip %q is already used by another load balancer", body.Spec.VIP)
		return
	}

	dbaasDb, m, err := controller.CreateIntoDb(r.Context(), body.General.ProductName, model.ProductTypeDBaaS.Name, azDb.Code, orgDb.ID, projectDb.ID)
	if err != nil {
		log.Err(err).Msg("Failed to save product into database")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	newBody, err := argodbaas.CreateArgoApp(r.Context(), dbaasDb.ID.String(), azDb, body.Spec, subnet, m, dbaasConfig, nil, chart)
	if err != nil {
		ctrlutils.CleanDb(r.Context(), dbaasDb.ID)
		log.Err(err).Msg("Failed to create argo app")
		httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
		return
	}

	if err := h.argo.CreateApp(r.Context(), newBody); err != nil {
		ctrlutils.CleanDb(r.Context(), dbaasDb.ID)
		ctrlutils.HandleArgoError(w, r, err, consts.SpxResourceCreationFailureCode, consts.SpxResourceCreationFailure)
		return
	}

	controller.WriteCreateResponse(w, dbaasDb.EffectiveID)
}

// UpdateDBaaS
//
//	@Summary		Update DBaaS
//	@Description	Update the instances, storage, CPU, memory, minor version or external access of a PostgreSQL database.
//	@Description	The call returns once ArgoCD holds the new values, follow the rollout from the status of the database.
//	@Tags			v1, SPX Argo Ctrl
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path	string			true	"Organization ID"
//	@Param			az			path	string			true	"AZ Code"
//	@Param			projectId	path	string			true	"Project ID"
//	@Param			effectiveId	path	string			true	"DBaaS EID"
//	@Param			Body		body	UpdateDBaaSBody	true	"DBaaS info"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/dbaas/{effectiveId} [post]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSWrite]
func (h *Service) UpdateDBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, orgDb, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	var body UpdateDBaaSBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	productEid := chi.URLParam(r, "effectiveId")
	dbaasDb, err := controller.UpdateIntoDb(r.Context(), productEid, body.General.ProductName, model.ProductTypeDBaaS.Name, azDb.Code, projectDb.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to save product in database")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}
	// Databases without a DB row are owned by a customer repository.
	if dbaasDb.ID == uuid.Nil {
		httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
		return
	}

	m := spxId.Metadata{}
	if err = m.GenerateMetadata(projectDb.ID.String(), orgDb.ID.String(), dbaasDb.ID.String()); err != nil {
		log.Err(err).Msg("Failed to generate metadata")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}

	dbaasConfig, err := getDBaaSConfig(r.Context(), azDb, orgDb.ID.String(), projectDb.ID.String())
	if err != nil {
		log.Err(err).Msg("Failed to get dbaas configuration from AZ")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}

	app, appFound, err := h.fetchDBaaSApp(r.Context(), projectDb.ID.String(), productEid)
	if err != nil {
		log.Err(err).Msg("Failed to fetch app")
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("eid", productEid).Msg(consts.SpxProxyToAZFailure)
		return
	}
	if !appFound {
		log.Error().Str("eid", productEid).Msg("App not found")
		httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
		return
	}

	chart, supported := argodbaas.ChartForUpdate(app.Chart, app.Spec.Version, body.Spec.Version)
	if !supported {
		log.Error().Str("version", body.Spec.Version).Msg("PostgreSQL version not supported")
		httpError.Http(w, r, http.StatusBadRequest).Msg("PostgreSQL version not supported")
		return
	}

	newBody, err := argodbaas.CreateArgoApp(r.Context(), dbaasDb.ID.String(), azDb, body.Spec, app.Subnet, m, dbaasConfig, &app.Spec, chart)
	if err != nil {
		log.Err(err).Msg("Failed to create argo app")
		httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
		return
	}

	// We only apply the spec part
	if err := h.argo.UpdateApp(r.Context(), newBody.General.AppName, h.argo.Namespace(projectDb.ID.String()), newBody.Spec); err != nil {
		ctrlutils.HandleArgoError(w, r, err, consts.SpxResourceUpdateFailureCode, consts.SpxResourceUpdateFailure)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteDBaaS
//
//	@Summary		Delete DBaaS
//	@Description	Delete a PostgreSQL database, with its EIP, by effective ID
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"DBaaS EID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/dbaas/{effectiveId} [delete]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSWrite]
func (h *Service) DeleteDBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, _, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	if !ctrlutils.CheckProductBelongsToProject(w, r, projectDb.ID, azDb.Code) {
		return
	}

	productEId := chi.URLParam(r, "effectiveId")

	// A missing app is not an error: the DB row still has to go.
	deleteErr := h.argo.DeleteApp(r.Context(), argodbaas.AppName(productEId), h.argo.Namespace(projectDb.ID.String()))
	if deleteErr != nil && !k8serrors.IsNotFound(deleteErr) {
		ctrlutils.HandleArgoError(w, r, deleteErr, consts.SpxResourceDeletionFailureCode, consts.SpxResourceDeletionFailure)
		return
	}

	rowsAffected, err := product.DeleteByEIdAndAZCodeAndProject(productEId, azDb.Code, projectDb.ID)
	if err != nil {
		log.Err(err).Msg(consts.SpxResourceDeletionFailure)
		httpError.Http(w, r, consts.SpxResourceDeletionFailureCode).Msg(consts.SpxResourceDeletionFailure)
		return
	}
	if k8serrors.IsNotFound(deleteErr) && rowsAffected == 0 {
		httpError.Http(w, r, http.StatusNotFound).Str("eid", productEId).Msg(consts.SpxResourceNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// RestartDBaaS
//
//	@Summary		Restart DBaaS
//	@Description	Rolling restart of the instances of a PostgreSQL database, the primary last
//	@Tags			v1, Superphenix Controller
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"DBaaS EID"
//	@Success		200
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/dbaas/{effectiveId}/restart [post]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSWrite]
func (h *Service) RestartDBaaS(w http.ResponseWriter, r *http.Request) {
	h.redirectOwned(w, r)
}

// GetDBaaSCredentials
//
//	@Summary		Get DBaaS credentials
//	@Description	Get the application user credentials and the load balancer endpoints of a PostgreSQL database
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path		string				true	"Organization ID"
//	@Param			az			path		string				true	"AZ Code"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			effectiveId	path		string				true	"DBaaS EID"
//	@Success		200			{object}	DBaaSCredentials	"Credentials"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/dbaas/{effectiveId}/credentials [get]
//	@Security		Bearer[OrganizationRead, ProjectDBaaSWrite]
func (h *Service) GetDBaaSCredentials(w http.ResponseWriter, r *http.Request) {
	h.redirectOwned(w, r)
}

// redirectOwned proxies the request to the AZ controller once the database is known to belong to the project.
func (h *Service) redirectOwned(w http.ResponseWriter, r *http.Request) {
	azDb, _, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}
	if !ctrlutils.CheckProductBelongsToProject(w, r, projectDb.ID, azDb.Code) {
		return
	}
	ctrlutils.SimpleRedirect()(w, r)
}

// dbaasApp holds what the Argo Application of a database describes.
// Gitops is "true" or "false".
type dbaasApp struct {
	Spec   argodbaas.DBaaSSpec
	Subnet argodbaas.Subnet
	Gitops string
	Chart  config.RepoArgoAppConfig
}

// fetchDBaaSApp reads the Application of a database. found is false when it does not exist.
func (h *Service) fetchDBaaSApp(ctx context.Context, projectId, resourceEId string) (app dbaasApp, found bool, err error) {
	log := logger.GetLogger(ctx)

	appView, err := h.argo.GetApp(ctx, argodbaas.AppName(resourceEId), h.argo.Namespace(projectId))
	if k8serrors.IsNotFound(err) {
		return dbaasApp{}, false, nil
	}
	if err != nil {
		log.Error().Err(err).Str("effectiveId", resourceEId).Msg("Failed to get argo app")
		return dbaasApp{}, false, fmt.Errorf("failed to get argo app")
	}

	app.Spec, app.Subnet, err = argodbaas.ConvertAppToDBaaSSpec(appView, projectId)
	if err != nil {
		log.Error().Err(err).Str("effectiveId", resourceEId).Msg("Failed to read app spec")
		return dbaasApp{}, true, err
	}

	app.Gitops = view.AppToResource(appView).Gitops
	if app.Gitops == "" {
		app.Gitops = "false"
	}
	app.Chart = argodbaas.ChartFromApp(appView)

	return app, true, nil
}

// externalAccess returns the EIP of a database from its local ID.
func externalAccess(ctx context.Context, projectId, localId string) *DBaaSExternalAccess {
	eid, err := argodbaas.EipEffectiveID(projectId, localId)
	if err != nil {
		log := logger.GetLogger(ctx)
		log.Err(err).Str("localId", localId).Msg("Failed to compute the EIP effective ID")
		return nil
	}
	return &DBaaSExternalAccess{EipEId: eid}
}

// getDBaaSConfig fetches the AZ specific configuration.
func getDBaaSConfig(ctx context.Context, azDb config.AZConfig, orgId, projectId string) (argodbaas.DBaaSConfig, error) {
	var res argodbaas.DBaaSConfig
	url := fmt.Sprintf("%s/%s/%s/dbaas-config", azDb.ControllerUrl, orgId, projectId)
	err := ctrlutils.GetFromAZ(ctx, azDb, url, &res)
	return res, err
}

// resolveSubnet reads the local ID and the CIDR of a subnet from the AZ controller.
func resolveSubnet(ctx context.Context, azDb config.AZConfig, orgId, projectId, subnetEId string) (argodbaas.Subnet, error) {
	if subnetEId == "" {
		return argodbaas.Subnet{}, fmt.Errorf("subnet is required")
	}

	var res struct {
		ID     string `json:"id"`
		Subnet struct {
			Spec struct {
				CIDRBlock string `json:"cidrBlock"`
			} `json:"spec"`
		} `json:"subnet"`
	}
	url := fmt.Sprintf("%s/%s/%s/subnet/%s", azDb.ControllerUrl, orgId, projectId, subnetEId)
	if err := ctrlutils.GetFromAZ(ctx, azDb, url, &res); err != nil {
		return argodbaas.Subnet{}, err
	}
	if res.ID == "" || res.Subnet.Spec.CIDRBlock == "" {
		return argodbaas.Subnet{}, fmt.Errorf("subnet %s has no local ID or CIDR", subnetEId)
	}

	return argodbaas.Subnet{LocalID: res.ID, CIDR: res.Subnet.Spec.CIDRBlock}, nil
}

// combineListResult regroups the results of the AZ controllers and the database.
// Databases only known by an AZ are owned by a customer repository, or being deleted when not GitOps.
func combineListResult(concatResults map[string][]any, resources []model.Product) []DBaaSFullResponse {
	found := make(map[string]bool, len(resources))
	combineResults := make([]DBaaSFullResponse, 0)

	for azCode, results := range concatResults {
		for _, result := range results {
			mapResult, ok := result.(map[string]any)
			if !ok {
				continue
			}
			id := stringField(mapResult, "id")
			entry := DBaaSFullResponse{
				ProductResponse: ProductResponse{
					ID:          id,
					EId:         stringField(mapResult, "eid"),
					ProductName: stringField(mapResult, "productName"),
					CodeAZ:      azCode, // we use az code to handle instance under PRA
					Gitops:      stringField(mapResult, "gitops"),
				},
				Cluster:   mapResult["cluster"],
				Endpoints: mapResult["endpoints"],
			}
			if i := slices.IndexFunc(resources, func(p model.Product) bool { return p.ID.String() == id }); i >= 0 {
				entry.ProductName = resources[i].ProductName
				entry.ProductTypeId = resources[i].ProductTypeId
				found[id] = true
			} else if entry.Gitops != "true" {
				continue
			}
			combineResults = append(combineResults, entry)
		}
	}

	for _, p := range resources {
		if found[p.ID.String()] {
			continue
		}
		combineResults = append(combineResults, DBaaSFullResponse{
			ProductResponse: ProductResponse{
				ID:            p.ID.String(),
				EId:           p.EffectiveID,
				ProductName:   p.ProductName,
				CodeAZ:        p.CodeAZ,
				ProductTypeId: p.ProductTypeId,
				Gitops:        "false",
			},
		})
	}

	slices.SortFunc(combineResults, func(a, b DBaaSFullResponse) int {
		return controller.CompareProductResult(a.ProductResponse, b.ProductResponse)
	})

	return combineResults
}

func stringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// DTOs shared with / owned by the controller kit; aliased so handler code and
// swagger annotations reference them unqualified.
type (
	ProductResponse     = controller.ProductResponse
	DBaaSFullResponse   = controller.DBaaSFullResponse
	DBaaSExternalAccess = controller.DBaaSExternalAccess
)

type DBaaSAppSpecResponse struct {
	ProductResponse `json:",inline"`
	Spec            argodbaas.DBaaSSpec `json:"spec"`
}

// DBaaSCredentials documents the credentials served by the AZ controller.
type DBaaSCredentials struct {
	Username  string          `json:"username"`
	Password  string          `json:"password"`
	DBName    string          `json:"dbname"`
	Endpoints []DBaaSEndpoint `json:"endpoints"`
}

// DBaaSEndpoint is a load balancer address serving one CloudNative-PG service: rw, ro or r.
type DBaaSEndpoint struct {
	Role string `json:"role,omitempty"`
	Host string `json:"host"`
	Port int64  `json:"port"`
}

type CreateDBaaSBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
	} `json:"general"`
	Spec argodbaas.DBaaSSpec `json:"spec"`
}

type UpdateDBaaSBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
	} `json:"general"`
	Spec argodbaas.DBaaSSpec `json:"spec"`
}
