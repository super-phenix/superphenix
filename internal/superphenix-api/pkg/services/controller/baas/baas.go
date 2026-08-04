package baas

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	argobaas "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app/baas"
	ctrlutils "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/utils"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// ListBaaS
//
//	@Summary		Retrieve all BaaS
//	@Description	Retrieve all BaaS instances
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Success		200			{array}	BaaSFullResponse	"BaaS"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{projectId}/baas [get]
//	@Security		Bearer[OrganizationRead, ProjectBaaSRead]
func (h *Service) ListBaaS(w http.ResponseWriter, r *http.Request) {
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

	resourcesDb, err := product.FindAllByProjectIdAndResourceType(projectDb.ID.String(), model.ProductTypeBaaS)
	if err != nil {
		log.Err(err).
			Str("projectId", projectDb.ID.String()).
			Str("resourceType", model.ProductTypeBaaS).
			Msg(consts.SpxFindAllResourcesError)
		httpError.Http(w, r, consts.SpxFindAllResourcesErrorCode).Msg(consts.SpxFindAllResourcesError)
		return
	}

	mapResourceCheck := make(map[uuid.UUID]bool)
	resources := make([]model.Product, 0)
	for _, p := range resourcesDb {
		mapResourceCheck[p.ID] = false

		// Keep only resources from available azs
		for i := range urls {
			if p.CodeAZ == urls[i].Code {
				resources = append(resources, p)
			}
		}
	}

	combineResults := combineListResult(concatResults, resourcesDb, mapResourceCheck)

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(combineResults)
	if err != nil {
		log.Err(err).Msg(consts.SpxResponseParseFailure)
		httpError.Http(w, r, consts.SpxResponseParseFailureCode).Msg(consts.SpxResponseParseFailure)
		return
	}
	_, _ = w.Write(marshal)
}

// GetBaaS
//
//	@Summary		Get BaaS instance
//	@Description	Get BaaS instance by effective ID
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path		string				true	"Organization ID"
//	@Param			az			path		string				true	"AZ Code"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			effectiveId	path		string				true	"BaaS EID"
//	@Success		200			{object}	BaaSFullResponse	"BaaS"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/baas/{effectiveId} [get]
//	@Security		Bearer[OrganizationRead, ProjectBaaSRead]
func (h *Service) GetBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, _, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	if !ctrlutils.CheckProductBelongsToProject(w, r, projectEntity.ID, azDb.Code) {
		return
	}

	resp, err := proxy.SendProxy(r, azDb, config.ApiPrefix, http.NoBody)
	if err != nil {
		log.Error().Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		log.Error().Str("path", r.URL.Path).Str("status", resp.Status).Int("statusCode", resp.StatusCode).Str("az", azDb.Code).Msg("Request on superphenix-controller failed")
	}
	defer resp.Body.Close()

	resourceEId := chi.URLParam(r, "effectiveId")
	dbProduct, dbErr := product.FindByEId(resourceEId)
	var result BaaSFullResponse

	// If we didn't find spx-ctrl, but we got db info
	if (resp.StatusCode != http.StatusOK) && dbErr == nil {
		result = BaaSFullResponse{
			ProductResponse: ProductResponse{
				ID:            dbProduct.ID.String(),
				EId:           dbProduct.EffectiveID,
				ProductName:   dbProduct.ProductName,
				CodeAZ:        azDb.Code,
				ProductTypeId: dbProduct.ProductTypeId,
				Gitops:        "false",
			},
		}
	} else if resp.StatusCode == http.StatusOK {
		mapResult := ctrlutils.ReadResponse(resp).(map[string]interface{})
		// We only have spx-ctrl info
		if dbErr != nil {
			log.Info().Err(dbErr).Str("resourceEId", resourceEId).Msg("Resource not found in DB")

			result = BaaSFullResponse{
				ProductResponse: ProductResponse{
					ID:          mapResult["id"].(string),
					EId:         mapResult["eid"].(string),
					ProductName: mapResult["productName"].(string),
					CodeAZ:      azDb.Code,
					Gitops:      mapResult["gitops"].(string),
				},
				Backup: mapResult["backup"],
			}
		} else {
			//	We got both db and spx-ctrl info
			if dbProduct.ID.String() == mapResult["id"] {
				result = BaaSFullResponse{
					ProductResponse: ProductResponse{
						ID:            dbProduct.ID.String(),
						EId:           mapResult["eid"].(string),
						ProductName:   dbProduct.ProductName,
						CodeAZ:        azDb.Code,
						ProductTypeId: dbProduct.ProductTypeId,
						Gitops:        mapResult["gitops"].(string),
					},
					Backup: mapResult["backup"],
				}
			}
		}
	}

	// If we can't find either spx-ctrl or db info
	if (resp.StatusCode == http.StatusNotFound) && dbErr != nil {
		log.Error().Str("effectiveId", resourceEId).Msg(consts.SpxResourceNotFound)
		httpError.Http(w, r, http.StatusNotFound).Str("eid", resourceEId).Msg(consts.SpxResourceNotFound)
		return
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

// CreateBaaS
//
//	@Summary		Create BaaS
//	@Description	Create a new BaaS Backup
//	@Tags			v1, SPX Argo Ctrl
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path		string			true	"Organization ID"
//	@Param			az			path		string			true	"AZ Code"
//	@Param			projectId	path		string			true	"Project ID"
//	@Param			Body		body		CreateBaaSBody	true	"BaaS info"
//	@Success		200			{object}	controller.CreateResponse
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/baas [post]
//	@Security		Bearer[OrganizationRead, ProjectBaaSWrite]
func (h *Service) CreateBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, orgDb, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	var body CreateBaaSBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	baasDb, m, err := controller.CreateIntoDb(r.Context(), body.General.ProductName, model.ProductTypeBaaS, azDb.Code, orgDb.ID, projectDb.ID)
	if err != nil {
		log.Err(err).Msg("Failed to save product into database")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	if body.Spec.Scheduled && body.Spec.Type == argobaas.BackupTypeAll {
		canCreate, err := canCreateAllBaas(r.Context(), orgDb.ID.String(), projectDb.ID.String(), azDb)
		if err != nil {
			ctrlutils.CleanDb(r.Context(), baasDb.ID)
			log.Err(err).Msg("Failed to check baas limit")
			httpError.Http(w, r, http.StatusBadRequest).Msg(http.StatusText(http.StatusBadRequest))
			return
		}
		if !canCreate {
			ctrlutils.CleanDb(r.Context(), baasDb.ID)
			log.Error().Msg("Cannot create All Scoped Backup - limit reached")
			httpError.Http(w, r, http.StatusBadRequest).Msg("Cannot create All Scoped Backup - limit reached for this AZ")
			return
		}
	}

	newBody, err := argobaas.CreateArgoApp(r.Context(), baasDb.ID.String(), azDb, body.Spec, m, nil)
	if err != nil {
		ctrlutils.CleanDb(r.Context(), baasDb.ID)
		log.Err(err).Msg("Failed to create argo app")
		httpError.Http(w, r, http.StatusBadRequest).Msg(http.StatusText(http.StatusBadRequest))
		return

	}

	if err := h.argo.CreateApp(r.Context(), newBody); err != nil {
		ctrlutils.CleanDb(r.Context(), baasDb.ID)
		ctrlutils.HandleArgoError(w, r, err, consts.SpxResourceCreationFailureCode, consts.SpxResourceCreationFailure)
		return
	}

	controller.WriteCreateResponse(w, baasDb.EffectiveID)
}

// GetForUpdateBaaS
//
//	@Summary		Get BaaS App
//	@Description	Get BaaS App for update form
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path		string				true	"Organization ID"
//	@Param			az			path		string				true	"AZ Code"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			effectiveId	path		string				true	"BaaS EID"
//	@Success		200			{object}	BaaSFullResponse	"BaaS"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/baas/{effectiveId}/app [get]
//	@Security		Bearer[OrganizationRead, ProjectBaaSRead]
func (h *Service) GetForUpdateBaaS(w http.ResponseWriter, r *http.Request) {
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

	spec, isGitops, appFound, err := h.fetchBaaSApp(r.Context(), projectDb.ID.String(), resourceEId)
	if err != nil {
		log.Err(err).Msg("Failed to fetch app")
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("eid", resourceEId).Msg(consts.SpxProxyToAZFailure)
		return
	}

	// If we can't find either spx-ctrl or db info
	if !appFound || dbErr != nil {
		log.Error().Str("effectiveId", resourceEId).Msg(consts.SpxResourceNotFound)
		httpError.Http(w, r, http.StatusNotFound).Str("eid", resourceEId).Msg(consts.SpxResourceNotFound)
		return
	}

	result := AppSpecFullResponse{
		ProductResponse: ProductResponse{
			ID:            dbProduct.ID.String(),
			EId:           dbProduct.EffectiveID,
			ProductName:   dbProduct.ProductName,
			CodeAZ:        azDb.Code,
			ProductTypeId: dbProduct.ProductTypeId,
			Gitops:        isGitops,
		},
		Spec: spec,
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

// UpdateBaaS
//
//	@Summary		Update BaaS
//	@Description	Update a BaaS Backup
//	@Tags			v1, SPX Argo Ctrl
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path	string			true	"Organization ID"
//	@Param			az			path	string			true	"AZ Code"
//	@Param			projectId	path	string			true	"Project ID"
//	@Param			effectiveId	path	string			true	"BaaS EID"
//	@Param			Body		body	UpdateBaaSBody	true	"BaaS info"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/baas/{effectiveId} [post]
//	@Security		Bearer[OrganizationRead, ProjectBaaSWrite]
func (h *Service) UpdateBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, orgDb, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	var body UpdateBaaSBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	productEid := chi.URLParam(r, "effectiveId")
	baasDb, err := controller.UpdateIntoDb(r.Context(), productEid, body.General.ProductName, model.ProductTypeBaaS, azDb.Code, projectDb.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to save product in database")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}

	m := spxId.Metadata{}
	if err = m.GenerateMetadata(projectDb.ID.String(), orgDb.ID.String(), baasDb.ID.String()); err != nil {
		log.Err(err).Msg("Failed to generate metadata")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}

	spec, _, appFound, err := h.fetchBaaSApp(r.Context(), projectDb.ID.String(), productEid)
	if err != nil {
		log.Err(err).Msg("Failed to fetch app")
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("eid", productEid).Msg(consts.SpxProxyToAZFailure)
		return
	}

	if !appFound {
		log.Err(err).Msg("App not found")
		httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
		return
	}

	newBody, err := argobaas.CreateArgoApp(r.Context(), baasDb.ID.String(), azDb, body.Spec, m, &spec)
	if err != nil {
		log.Err(err).Msg("Failed to create argo app")
		httpError.Http(w, r, http.StatusBadRequest).Msg(http.StatusText(http.StatusBadRequest))
		return
	}

	// We only apply the spec part
	appName := fmt.Sprintf("%s-%s", argobaas.AppPrefix, baasDb.EffectiveID)
	if err := h.argo.UpdateApp(r.Context(), appName, h.argo.Namespace(projectDb.ID.String()), newBody.Spec); err != nil {
		ctrlutils.HandleArgoError(w, r, err, consts.SpxResourceUpdateFailureCode, consts.SpxResourceUpdateFailure)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteBaaS
//
//	@Summary		Delete BaaS
//	@Description	Delete BaaS Instance by effective ID
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"BaaS EID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/baas/{effectiveId} [delete]
//	@Security		Bearer[OrganizationRead, ProjectBaaSWrite]
func (h *Service) DeleteBaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, orgDb, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	if !ctrlutils.CheckProductBelongsToProject(w, r, projectDb.ID, azDb.Code) {
		return
	}

	productEId := chi.URLParam(r, "effectiveId")

	appName := fmt.Sprintf("%s-%s", argobaas.AppPrefix, productEId)
	// A missing app is not an error: the backup and the DB row still have to go.
	deleteErr := h.argo.DeleteApp(r.Context(), appName, h.argo.Namespace(projectDb.ID.String()))
	if deleteErr != nil && !k8serrors.IsNotFound(deleteErr) {
		ctrlutils.HandleArgoError(w, r, deleteErr, consts.SpxResourceDeletionFailureCode, consts.SpxResourceDeletionFailure)
		return
	}

	// We need to ask Controller to clean remaining backup
	url := fmt.Sprintf("%s/%s/%s/baas/%s", azDb.ControllerUrl, orgDb.ID.String(), projectDb.ID.String(), productEId)
	resp2, err := proxy.SendRequest(r.Context(), url, "DELETE", http.NoBody, azDb.AuthSecret)
	if err != nil {
		log.Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
		httpError.Http(w, r, consts.SpxResourceDeletionFailureCode).Msg(consts.SpxResourceDeletionFailure)
		return
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 && resp2.StatusCode != 404 {
		log.Err(err).
			Str("az", azDb.Code).
			Str("status", resp2.Status).
			Int("statusCode", resp2.StatusCode).
			Msg("Failed to delete baas on controller AZ")
		httpError.Http(w, r, consts.SpxResourceDeletionFailureCode).Msg(consts.SpxResourceDeletionFailure)
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

// fetchBaaSApp fetch the application status and spec from argo controller
//
// Returns:
//   - spec: The specification of the BaaS application.
//   - isGitops: A string ("true" or "false") indicating if the application is managed via GitOps.
//   - found: A boolean indicating if the application was found in the Argo controller.
//   - err: An error object if the request failed.
func (h *Service) fetchBaaSApp(ctx context.Context, projectId, resourceEId string) (spec argobaas.BaaSSpec, isGitops string, found bool, err error) {
	log := logger.GetLogger(ctx)

	appName := fmt.Sprintf("%s-%s", argobaas.AppPrefix, resourceEId)
	appView, err := h.argo.GetApp(ctx, appName, h.argo.Namespace(projectId))
	if k8serrors.IsNotFound(err) {
		return argobaas.BaaSSpec{}, "", false, nil
	}
	if err != nil {
		log.Error().Err(err).Str("effectiveId", resourceEId).Msg("Failed to get argo app")
		return argobaas.BaaSSpec{}, "false", false, fmt.Errorf("failed to get argo app")
	}

	spec, err = argobaas.ConvertAppToUpdateBaaSSpec(appView)
	if err != nil {
		log.Error().Str("effectiveId", resourceEId).Msg("Failed to read app spec")
		return argobaas.BaaSSpec{}, "", true, err
	}

	isGitops = view.AppToResource(appView).Gitops
	if isGitops == "" {
		isGitops = "false"
	}

	return spec, isGitops, true, nil
}

func canCreateAllBaas(ctx context.Context, orgId, projectId string, az config.AZConfig) (bool, error) {
	log := logger.GetLogger(ctx)
	url := fmt.Sprintf("%s/%s/%s/baas/check-limit", az.ControllerUrl, orgId, projectId)
	resp, err := proxy.SendRequest(ctx, url, "GET", http.NoBody, az.AuthSecret)
	if err != nil {
		log.Err(err).Str("az", az.Code).Msg(consts.SpxProxyToAZFailure)
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errorBody := httpError.RetrieveHttpError(resp)
		log.Error().Any("responseBody", errorBody).
			Str("status", resp.Status).Int("statusCode", resp.StatusCode).
			Str("azCode", az.Code).
			Msg("Failed to check baas limit")
	}

	// read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Error reading response body")
		return false, fmt.Errorf("error reading response body")
	}

	var res struct {
		CanCreate bool `json:"canCreate"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		log.Error().Err(err).Str("request-url", resp.Request.URL.String()).Msg("Failed to unmarshal result")
		return false, fmt.Errorf("error unmarshaling result")
	}

	return res.CanCreate, nil
}

// combineListResult regroup results from db and controller
func combineListResult(concatResults map[string][]interface{}, resources []model.Product, mapResourceCheck map[uuid.UUID]bool) []BaaSFullResponse {
	combineResults := make([]BaaSFullResponse, 0)
	for azCode, results := range concatResults {
		for _, result := range results {
			mapResult := result.(map[string]interface{})
			found := false
			for _, p := range resources {
				if p.ID.String() == mapResult["id"] {
					combineResults = append(combineResults, BaaSFullResponse{
						ProductResponse: ProductResponse{
							ID:            p.ID.String(),
							EId:           mapResult["eid"].(string),
							ProductName:   p.ProductName,
							CodeAZ:        azCode, // we use az code to handle instance under PRA
							ProductTypeId: p.ProductTypeId,
							Gitops:        mapResult["gitops"].(string),
						},
						Backup: mapResult["backup"],
					})
					mapResourceCheck[p.ID] = true
					found = true
					break
				}
			}

			// If only gitops
			if !found {
				combineResults = append(combineResults, BaaSFullResponse{
					ProductResponse: ProductResponse{
						ID:          mapResult["id"].(string),
						EId:         mapResult["eid"].(string),
						ProductName: mapResult["productName"].(string),
						CodeAZ:      azCode,
						Gitops:      mapResult["gitops"].(string),
					},
					Backup: mapResult["backup"],
				})
			}
		}

	}

	// Check for not found resources
	for _, p := range resources {
		if mapResourceCheck[p.ID] == false {
			combineResults = append(combineResults, BaaSFullResponse{
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
	}

	slices.SortFunc(combineResults, func(a, b BaaSFullResponse) int {
		return controller.CompareProductResult(a.ProductResponse, b.ProductResponse)
	})

	return combineResults
}

// DTOs shared with / owned by the controller kit; aliased so handler code and
// swagger annotations reference them unqualified.
type (
	ProductResponse     = controller.ProductResponse
	BaaSFullResponse    = controller.BaaSFullResponse
	AppSpecFullResponse = controller.AppSpecFullResponse
)

type CreateBaaSBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
	} `json:"general"`
	Spec argobaas.BaaSSpec `json:"spec"`
}

type UpdateBaaSBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
	} `json:"general"`
	Spec argobaas.BaaSSpec `json:"spec"`
}
