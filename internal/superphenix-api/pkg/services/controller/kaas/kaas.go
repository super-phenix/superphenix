package kaas

import (
	"bytes"
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
	argokaas "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app/kaas"
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

// ListKaaS
//
//	@Summary		Retrieve all KaaS
//	@Description	Retrieve all KaaS instances
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path	string				true	"Organization ID"
//	@Param			az			path	string				true	"AZ Code"
//	@Param			projectId	path	string				true	"Project ID"
//	@Success		200			{array}	KaaSFullResponse	"KaaS"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{projectId}/kaas [get]
//	@Security		Bearer[OrganizationRead, ProjectKaaSRead]
func (h *Service) ListKaaS(w http.ResponseWriter, r *http.Request) {
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

	resourcesDb, err := product.FindAllByProjectIdAndResourceType(projectDb.ID.String(), model.ProductTypeKaaS)
	if err != nil {
		log.Err(err).
			Str("projectId", projectDb.ID.String()).
			Str("resourceType", model.ProductTypeKaaS).
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

// GetKaaS
//
//	@Summary		Get KaaS instance
//	@Description	Get KaaS instance by effective ID
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path		string				true	"Organization ID"
//	@Param			az			path		string				true	"AZ Code"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			effectiveId	path		string				true	"KaaS EID"
//	@Success		200			{object}	KaaSFullResponse	"KaaS"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/kaas/{effectiveId} [get]
//	@Security		Bearer[OrganizationRead, ProjectKaaSRead]
func (h *Service) GetKaaS(w http.ResponseWriter, r *http.Request) {
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
	var result KaaSFullResponse

	// If we didn't find spx-ctrl, but we got db info
	if (resp.StatusCode != http.StatusOK) && dbErr == nil {
		result = KaaSFullResponse{
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

			result = KaaSFullResponse{
				ProductResponse: ProductResponse{
					ID:          mapResult["id"].(string),
					EId:         mapResult["eid"].(string),
					ProductName: mapResult["productName"].(string),
					CodeAZ:      azDb.Code,
					Gitops:      mapResult["gitops"].(string),
				},
				Cluster: mapResult["cluster"],
			}
		} else {
			//	We got both db and spx-ctrl info
			if dbProduct.ID.String() == mapResult["id"] {
				result = KaaSFullResponse{
					ProductResponse: ProductResponse{
						ID:            dbProduct.ID.String(),
						EId:           mapResult["eid"].(string),
						ProductName:   dbProduct.ProductName,
						CodeAZ:        azDb.Code,
						ProductTypeId: dbProduct.ProductTypeId,
						Gitops:        mapResult["gitops"].(string),
					},
					Cluster: mapResult["cluster"],
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

// CreateKaaS
//
//	@Summary		Create KaaS
//	@Description	Create a new KaaS Cluster
//	@Tags			v1, SPX Argo Ctrl
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path		string			true	"Organization ID"
//	@Param			az			path		string			true	"AZ Code"
//	@Param			projectId	path		string			true	"Project ID"
//	@Param			Body		body		CreateKaaSBody	true	"KaaS info"
//	@Success		200			{object}	controller.CreateResponse
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/kaas [post]
//	@Security		Bearer[OrganizationRead, ProjectKaaSWrite]
func (h *Service) CreateKaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, orgDb, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	// Fetch kaas az config
	kaasConfig, err := getKaaSConfig(r.Context(), azDb, orgDb.ID.String(), projectDb.ID.String())
	if err != nil {
		log.Err(err).Msg("Failed to get kaas configuration from AZ")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	var body CreateKaaSBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	kaasDb, m, err := controller.CreateIntoDb(r.Context(), body.General.ProductName, model.ProductTypeKaaS, azDb.Code, orgDb.ID, projectDb.ID)
	if err != nil {
		log.Err(err).Msg("Failed to save product into database")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	newBody, _, err := argokaas.CreateArgoApp(r.Context(), kaasDb.ID.String(), azDb, body.Spec, m, kaasConfig, nil)
	if err != nil {
		ctrlutils.CleanDb(r.Context(), kaasDb.ID)
		log.Err(err).Msg("Failed to create argo app")
		httpError.Http(w, r, http.StatusBadRequest).Msg(http.StatusText(http.StatusBadRequest))
		return

	}

	if err := h.argo.CreateApp(r.Context(), newBody); err != nil {
		ctrlutils.CleanDb(r.Context(), kaasDb.ID)
		ctrlutils.HandleArgoError(w, r, err, consts.SpxResourceCreationFailureCode, consts.SpxResourceCreationFailure)
		return
	}

	controller.WriteCreateResponse(w, kaasDb.EffectiveID)
}

// GetForUpdateKaaS
//
//	@Summary		Get KaaS App
//	@Description	Get KaaS App for update form
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path		string				true	"Organization ID"
//	@Param			az			path		string				true	"AZ Code"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			effectiveId	path		string				true	"KaaS EID"
//	@Success		200			{object}	KaaSFullResponse	"KaaS"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/kaas/{effectiveId}/app [get]
//	@Security		Bearer[OrganizationRead, ProjectKaaSRead]
func (h *Service) GetForUpdateKaaS(w http.ResponseWriter, r *http.Request) {
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

	spec, isGitops, appFound, err := h.fetchKaaSApp(r.Context(), projectDb.ID.String(), resourceEId)
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

// UpdateKaaS
//
//	@Summary		Update KaaS
//	@Description	Update a KaaS Cluster
//	@Tags			v1, SPX Argo Ctrl
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path	string			true	"Organization ID"
//	@Param			az			path	string			true	"AZ Code"
//	@Param			projectId	path	string			true	"Project ID"
//	@Param			Body		body	UpdateKaaSBody	true	"KaaS info"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/kaas/{effectiveId} [post]
//	@Security		Bearer[OrganizationRead, ProjectKaaSWrite]
func (h *Service) UpdateKaaS(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, orgDb, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	// Fetch kaas az config
	kaasConfig, err := getKaaSConfig(r.Context(), azDb, orgDb.ID.String(), projectDb.ID.String())
	if err != nil {
		log.Err(err).Msg("Failed to get kaas configuration from AZ")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	var body UpdateKaaSBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	productEid := chi.URLParam(r, "effectiveId")
	kaasDb, err := controller.UpdateIntoDb(r.Context(), productEid, body.General.ProductName, model.ProductTypeKaaS, azDb.Code, projectDb.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to save product in database")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}

	m := spxId.Metadata{}
	if err = m.GenerateMetadata(projectDb.ID.String(), orgDb.ID.String(), kaasDb.ID.String()); err != nil {
		log.Err(err).Msg("Failed to generate metadata")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}

	spec, _, appFound, err := h.fetchKaaSApp(r.Context(), projectDb.ID.String(), productEid)
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

	newBody, gtr, err := argokaas.CreateArgoApp(r.Context(), kaasDb.ID.String(), azDb, body.Spec, m, kaasConfig, &spec)
	if err != nil {
		log.Err(err).Msg("Failed to create argo app")
		httpError.Http(w, r, http.StatusBadRequest).Msg(http.StatusText(http.StatusBadRequest))
		return
	}

	// We only apply the spec part
	appName := fmt.Sprintf("%s-%s", argokaas.KaasPrefix, kaasDb.EffectiveID)
	if err := h.argo.UpdateApp(r.Context(), appName, h.argo.Namespace(projectDb.ID.String()), newBody.Spec); err != nil {
		ctrlutils.HandleArgoError(w, r, err, consts.SpxResourceUpdateFailureCode, consts.SpxResourceUpdateFailure)
		return
	}

	// Check for group to remove and send request to delete them to the Superphenix Controller
	// We have to do this manually because ArgoCD cannot clean them because they have a OwnerReference
	if len(gtr) > 0 {
		url := fmt.Sprintf("%s/%s/%s/kaas/%s/delete-group", azDb.ControllerUrl, orgDb.ID.String(), projectDb.ID.String(), productEid)

		gdBody, err := json.Marshal(argokaas.GroupDeletion{GroupName: gtr})
		if err != nil {
			log.Err(err).Msg("Failed to marshal group deletion body")
		} else {
			resp2, err := proxy.SendRequest(r.Context(), url, "POST", bytes.NewReader(gdBody), azDb.AuthSecret)
			if err != nil {
				log.Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
				return
			}
			defer resp2.Body.Close()
			if resp2.StatusCode != 200 {
				log.Err(err).Str("az", azDb.Code).Str("status", resp2.Status).Int("statusCode", resp2.StatusCode).Msg(consts.SpxProxyToAZFailure)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// ReinstallKaaSEssentials
//
//	@Summary		Reinstall KaaS Essentials
//	@Description	Reinstall KaaS Essentials by incrementing the kaasEssentials revision
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"KaaS EID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/kaas/{effectiveId}/reinstall-essentials [get]
//	@Security		Bearer[OrganizationRead, ProjectKaaSWrite]
func (h *Service) ReinstallKaaSEssentials(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, orgDb, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	// Fetch kaas az config
	kaasConfig, err := getKaaSConfig(r.Context(), azDb, orgDb.ID.String(), projectDb.ID.String())
	if err != nil {
		log.Err(err).Msg("Failed to get kaas configuration from AZ")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}

	productEid := chi.URLParam(r, "effectiveId")

	kaasDb, err := product.FindByEId(productEid)
	if err != nil {
		log.Error().Err(err).Msg("Failed to find product in database")
		httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
		return
	}

	if kaasDb.ProjectId != projectDb.ID {
		log.Error().Str("eid", productEid).Msg("Product does not belong to this project")
		httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
		return
	}

	m := spxId.Metadata{}
	if err = m.GenerateMetadata(projectDb.ID.String(), orgDb.ID.String(), kaasDb.ID.String()); err != nil {
		log.Err(err).Msg("Failed to generate metadata")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}

	spec, _, appFound, err := h.fetchKaaSApp(r.Context(), projectDb.ID.String(), productEid)
	if err != nil {
		log.Err(err).Msg("Failed to fetch app")
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("eid", productEid).Msg(consts.SpxProxyToAZFailure)
		return
	}

	if !appFound {
		log.Error().Msg("App not found")
		httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
		return
	}

	// Increment the essentials revision to trigger a reinstallation
	spec.KaasEssentials.Revision++

	newBody, _, err := argokaas.CreateArgoApp(r.Context(), kaasDb.ID.String(), azDb, spec, m, kaasConfig, &spec)
	if err != nil {
		log.Err(err).Msg("Failed to create argo app")
		httpError.Http(w, r, http.StatusBadRequest).Msg(http.StatusText(http.StatusBadRequest))
		return
	}

	appName := fmt.Sprintf("%s-%s", argokaas.KaasPrefix, kaasDb.EffectiveID)
	if err := h.argo.UpdateApp(r.Context(), appName, h.argo.Namespace(projectDb.ID.String()), newBody.Spec); err != nil {
		ctrlutils.HandleArgoError(w, r, err, consts.SpxResourceUpdateFailureCode, consts.SpxResourceUpdateFailure)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteKaaS
//
//	@Summary		Delete KaaS
//	@Description	Delete KaaS Instance by effective ID
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"KaaS EID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/kaas/{effectiveId} [delete]
//	@Security		Bearer[OrganizationRead, ProjectKaaSWrite]
func (h *Service) DeleteKaaS(w http.ResponseWriter, r *http.Request) {
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

	appName := fmt.Sprintf("%s-%s", argokaas.KaasPrefix, productEId)
	// A missing app is not an error: the DB row still has to go.
	deleteErr := h.argo.DeleteApp(r.Context(), appName, h.argo.Namespace(projectDb.ID.String()))
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

// GetKubeVersion
//
//	@Summary		Get supported KubeVersions
//	@Description	Get supported KubeVersions
//	@Tags			v1, SPX Argo Ctrl
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			projectId	path	string	true	"Project ID"
//	@Success		200			{array}	string	"Kube Versions"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{projectId}/kaas/kube-versions [get]
//	@Security		Bearer[OrganizationRead]
func (h *Service) GetKubeVersion(w http.ResponseWriter, r *http.Request) {
	kubeVersions := h.cfg.ProductsConfig.ArgoApp.Kubernetes.KubeVersions
	versions := make([]string, 0, len(kubeVersions))
	for _, v := range kubeVersions {
		versions = append(versions, v.Version)
	}
	b, _ := json.Marshal(versions)
	ch.Data(w, http.StatusOK, ch.MIMEJSON, b)
}

// GetKaaSKubeConfig
//
//	@Summary		Get KaaS KubeConfig
//	@Description	Retrieve KubeConfig for a KaaS cluster
//	@Tags			v1, Superphenix Controller
//	@Produce		octet-stream
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"KaaS EID"
//	@Success		200			{file}	binary	"KubeConfig file"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/kaas/{effectiveId}/kubeconfig [get]
//	@Security		Bearer[OrganizationRead, ProjectKaaSRead, ProjectKaaSKubeConfig]
func (h *Service) GetKaaSKubeConfig(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, orgDb, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	effectiveId := chi.URLParam(r, "effectiveId")

	// Fetch the raw kubeconfig from the AZ controller.
	url := fmt.Sprintf("%s/%s/%s/kaas/%s/kubeconfig", azDb.ControllerUrl, orgDb.ID.String(), projectDb.ID.String(), effectiveId)
	resp, err := proxy.SendRequest(r.Context(), url, "GET", http.NoBody, azDb.AuthSecret)
	if err != nil {
		log.Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("eid", effectiveId).Msg(consts.SpxProxyToAZFailure)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		httpError.Http(w, r, http.StatusNotFound).Str("eid", effectiveId).Msg(consts.SpxResourceNotFound)
		return
	}
	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status", resp.StatusCode).Str("eid", effectiveId).Msg("Failed to fetch kubeconfig from AZ")
		httpError.Http(w, r, http.StatusInternalServerError).Str("eid", effectiveId).Msg(consts.SpxProxyToAZFailure)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Err(err).Str("eid", effectiveId).Msg("Failed to read kubeconfig response")
		httpError.Http(w, r, http.StatusInternalServerError).Str("eid", effectiveId).Msg(consts.SpxProxyToAZFailure)
		return
	}

	// The kube version is authoritative on the Argo app, not the served kubeconfig.
	spec, _, appFound, err := h.fetchKaaSApp(r.Context(), projectDb.ID.String(), effectiveId)
	if err != nil {
		log.Err(err).Msg("Failed to fetch app")
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("eid", effectiveId).Msg(consts.SpxProxyToAZFailure)
		return
	}
	if !appFound {
		httpError.Http(w, r, http.StatusNotFound).Str("eid", effectiveId).Msg(consts.SpxResourceNotFound)
		return
	}

	if argokaas.ShouldRewriteFQDN(h.cfg.ProductsConfig.ArgoApp.Kubernetes.KubeVersions, spec.KubeVersion) {
		body, err = argokaas.RewriteFQDN(body, effectiveId, azDb.Code, h.cfg.ProductsConfig.ArgoApp.Kubernetes.KubeConfigDomain)
		if err != nil {
			log.Err(err).Str("eid", effectiveId).Msg("Failed to rewrite kubeconfig server endpoint")
			httpError.Http(w, r, http.StatusInternalServerError).Str("eid", effectiveId).Msg(consts.SpxResponseParseFailure)
			return
		}
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename=".kubeconfig"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// getKaaSConfig fetch the AZ specific configuration
func getKaaSConfig(ctx context.Context, az config.AZConfig, orgId, projectId string) (argokaas.KaaSConfig, error) {
	log := logger.GetLogger(ctx)
	url := fmt.Sprintf("%s/%s/%s/kaas-config", az.ControllerUrl, orgId, projectId)
	resp, err := proxy.SendRequest(ctx, url, "GET", http.NoBody, az.AuthSecret)
	if err != nil {
		log.Err(err).Str("az", az.Code).Msg(consts.SpxProxyToAZFailure)
		return argokaas.KaaSConfig{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errorBody := httpError.RetrieveHttpError(resp)
		log.Error().Any("responseBody", errorBody).
			Str("status", resp.Status).Int("statusCode", resp.StatusCode).
			Str("azCode", az.Code).
			Msg("Failed to get KaaS config")
	}

	// read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Error reading response body")
		return argokaas.KaaSConfig{}, fmt.Errorf("error reading response body")
	}

	var res argokaas.KaaSConfig
	if err := json.Unmarshal(body, &res); err != nil {
		log.Error().Err(err).Str("request-url", resp.Request.URL.String()).Msg("Failed to unmarshal kaas config")
		return argokaas.KaaSConfig{}, fmt.Errorf("error unmarshaling kaas config")
	}

	return res, nil
}

// fetchKaaSApp fetch the application status and spec from argo controller
//
// Returns:
//   - spec: The specification of the KaaS application.
//   - isGitops: A string ("true" or "false") indicating if the application is managed via GitOps.
//   - found: A boolean indicating if the application was found in the Argo controller.
//   - err: An error object if the request failed.
func (h *Service) fetchKaaSApp(ctx context.Context, projectId, resourceEId string) (spec argokaas.KaaSSpec, isGitops string, found bool, err error) {
	log := logger.GetLogger(ctx)

	appName := fmt.Sprintf("%s-%s", argokaas.KaasPrefix, resourceEId)
	appView, err := h.argo.GetApp(ctx, appName, h.argo.Namespace(projectId))
	if k8serrors.IsNotFound(err) {
		return argokaas.KaaSSpec{}, "", false, nil
	}
	if err != nil {
		log.Error().Err(err).Str("effectiveId", resourceEId).Msg("Failed to get argo app")
		return argokaas.KaaSSpec{}, "false", false, fmt.Errorf("failed to get argo app")
	}

	spec, err = argokaas.ConvertAppToUpdateKaaSSpec(appView)
	if err != nil {
		log.Error().Str("effectiveId", resourceEId).Msg("Failed to read app spec")
		return argokaas.KaaSSpec{}, "", true, err
	}

	isGitops = view.AppToResource(appView).Gitops
	if isGitops == "" {
		isGitops = "false"
	}

	return spec, isGitops, true, nil
}

// combineListResult regroup results from db and controller
func combineListResult(concatResults map[string][]interface{}, resources []model.Product, mapResourceCheck map[uuid.UUID]bool) []KaaSFullResponse {
	combineResults := make([]KaaSFullResponse, 0)
	for azCode, results := range concatResults {
		for _, result := range results {
			mapResult := result.(map[string]interface{})
			found := false
			for _, p := range resources {
				if p.ID.String() == mapResult["id"] {
					combineResults = append(combineResults, KaaSFullResponse{
						ProductResponse: ProductResponse{
							ID:            p.ID.String(),
							EId:           mapResult["eid"].(string),
							ProductName:   p.ProductName,
							CodeAZ:        azCode, // we use az code to handle instance under PRA
							ProductTypeId: p.ProductTypeId,
							Gitops:        mapResult["gitops"].(string),
						},
						Cluster: mapResult["cluster"],
					})
					mapResourceCheck[p.ID] = true
					found = true
					break
				}
			}

			// If only gitops
			if !found {
				combineResults = append(combineResults, KaaSFullResponse{
					ProductResponse: ProductResponse{
						ID:          mapResult["id"].(string),
						EId:         mapResult["eid"].(string),
						ProductName: mapResult["productName"].(string),
						CodeAZ:      azCode,
						Gitops:      mapResult["gitops"].(string),
					},
					Cluster: mapResult["cluster"],
				})
			}
		}

	}

	// Check for not found resources
	for _, p := range resources {
		if mapResourceCheck[p.ID] == false {
			combineResults = append(combineResults, KaaSFullResponse{
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

	slices.SortFunc(combineResults, func(a, b KaaSFullResponse) int {
		return controller.CompareProductResult(a.ProductResponse, b.ProductResponse)
	})

	return combineResults
}

// DTOs owned by the controller kit; aliased so handler code and swagger
// annotations reference them unqualified.
type (
	ProductResponse     = controller.ProductResponse
	KaaSFullResponse    = controller.KaaSFullResponse
	AppSpecFullResponse = controller.AppSpecFullResponse
)

type CreateKaaSBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
	} `json:"general"`
	Spec argokaas.KaaSSpec `json:"spec"`
}

type UpdateKaaSBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
	} `json:"general"`
	Spec argokaas.KaaSSpec `json:"spec"`
}

// Instances
//
//	@Summary		Get KaaS Instances
//	@Description	Retrieve instances belonging to a KaaS cluster
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string							true	"Organization ID"
//	@Param			az			path	string							true	"AZ Code"
//	@Param			projectId	path	string							true	"Project ID"
//	@Param			effectiveId	path	string							true	"KaaS EID"
//	@Success		200			{array}	instance.InstanceFullResponse	"Instances"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/kaas/{effectiveId}/instances [get]
//	@Security		Bearer[OrganizationRead, ProjectKaaSRead, ProjectInstanceRead]
func (h *Service) Instances(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}

// Netpols
//
//	@Summary		Get KaaS network policies
//	@Description	Retrieve the network policies of a KaaS cluster
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"KaaS EID"
//	@Success		200
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/kaas/{effectiveId}/netpols [get]
//	@Security		Bearer[OrganizationRead, ProjectKaaSRead, ProjectFirewallRead]
func (h *Service) Netpols(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}
