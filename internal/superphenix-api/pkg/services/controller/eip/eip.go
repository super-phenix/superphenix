package eip

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/az"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/product"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/proxy"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"
	ctrlutils "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/utils"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListEips
//
//	@Summary		Retrieve all EIPs
//	@Description	Retrieve all EIPs across AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string			true	"Organization ID"
//	@Param			projectId	path	string			true	"Project ID"
//	@Success		200			{array}	EipFullResponse	"EIPs"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{projectId}/eip [get]
//	@Security		Bearer[OrganizationRead, ProjectEipRead]
func (h *Service) ListEips(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orga, project, code, errMsg := ctrlutils.CheckListPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	urls := az.FindAll(orga.ID.String())

	responses, err := proxy.SendBatchProxy(r, urls, config.ApiPrefix)
	if err != nil {
		log.Err(err).Msg(consts.SpxProxyToAZFailure)
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Msg(consts.SpxProxyToAZFailure)
		return
	}
	concatResults := ctrlutils.ConcatResponses(r.Context(), responses)

	resourcesDb, err := product.FindAllByProjectIdAndResourceType(project.ID.String(), model.ProductTypeEIP)
	if err != nil {
		log.Err(err).Str("projectId", project.ID.String()).Str("resourceType", model.ProductTypeEIP).Msg(consts.SpxFindAllResourcesError)
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

	combineResults := combineListResult(concatResults, resources, mapResourceCheck)

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(combineResults)
	if err != nil {
		log.Err(err).Msg(consts.SpxResponseParseFailure)
		httpError.Http(w, r, consts.SpxResponseParseFailureCode).Msg(consts.SpxResponseParseFailure)
		return
	}
	_, _ = w.Write(marshal)
}

// ListAZEips
//
//	@Summary		Retrieve all AZ EIPs
//	@Description	Retrieve all EIPs for a specific AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string			true	"Organization ID"
//	@Param			az			path	string			true	"AZ Code"
//	@Param			projectId	path	string			true	"Project ID"
//	@Success		200			{array}	EipFullResponse	"Eips"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/eip [get]
//	@Security		Bearer[OrganizationRead, ProjectEipRead]
func (h *Service) ListAZEips(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, _, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	resp, err := proxy.SendProxy(r, azDb, config.ApiPrefix, http.NoBody)
	if err != nil {
		log.Error().Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		log.Error().Str("path", r.URL.Path).Str("status", resp.Status).Int("statusCode", resp.StatusCode).Str("az", azDb.Code).Msg("Request on superphenix-controller failed")
	}
	concatResults := ctrlutils.ConcatResponses(r.Context(), map[string]*http.Response{azDb.Code: resp})

	products, err := product.FindAllByProjectIdAndResourceTypeAndCodeAZ(projectDb.ID.String(), model.ProductTypeEIP, azDb.Code)
	if err != nil {
		log.Err(err).
			Str("projectId", projectDb.ID.String()).
			Str("resourceType", model.ProductTypeEIP).
			Msg(consts.SpxFindAllResourcesError)
		httpError.Http(w, r, consts.SpxFindAllResourcesErrorCode).Msg(consts.SpxFindAllResourcesError)
		return
	}

	mapResourceCheck := make(map[uuid.UUID]bool)
	for _, p := range products {
		mapResourceCheck[p.ID] = false
	}

	combineResults := combineListResult(concatResults, products, mapResourceCheck)

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(combineResults)
	if err != nil {
		log.Err(err).Msg(consts.SpxResponseParseFailure)
		httpError.Http(w, r, consts.SpxResponseParseFailureCode).Msg(consts.SpxResponseParseFailure)
		return
	}
	_, _ = w.Write(marshal)
}

// GetEip
//
//	@Summary		Get EIP
//	@Description	Get EIP by Effective ID
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path		string			true	"Organization ID"
//	@Param			az			path		string			true	"AZ Code"
//	@Param			projectId	path		string			true	"Project ID"
//	@Param			effectiveId	path		string			true	"Eip EID"
//	@Success		200			{object}	EipFullResponse	"Eip"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/eip/{effectiveId} [get]
//	@Security		Bearer[OrganizationRead, ProjectEipRead]
func (h *Service) GetEip(w http.ResponseWriter, r *http.Request) {
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

	productEId := chi.URLParam(r, "effectiveId")
	dbProduct, err := product.FindByEId(productEId)
	var result EipFullResponse

	if (resp.StatusCode != http.StatusOK) && err == nil {
		result = EipFullResponse{
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
		if err != nil {
			log.Info().Err(err).Str("productEId", productEId).Msg("Resource not found in DB")

			result = EipFullResponse{
				ProductResponse: ProductResponse{
					ID:          mapResult["id"].(string),
					EId:         mapResult["eid"].(string),
					ProductName: mapResult["productName"].(string),
					CodeAZ:      azDb.Code,
					Gitops:      mapResult["gitops"].(string),
				},
				Eip:  mapResult["eip"],
				Fip:  mapResult["fip"],
				SNat: mapResult["snat"],
				DNat: mapResult["dnat"],
			}
		} else {
			//	We got both db and spx-ctrl info
			if dbProduct.ID.String() == mapResult["id"] {
				result = EipFullResponse{
					ProductResponse: ProductResponse{
						ID:            dbProduct.ID.String(),
						EId:           mapResult["eid"].(string),
						ProductName:   dbProduct.ProductName,
						CodeAZ:        azDb.Code,
						ProductTypeId: dbProduct.ProductTypeId,
						Gitops:        mapResult["gitops"].(string),
					},
					Eip:  mapResult["eip"],
					Fip:  mapResult["fip"],
					SNat: mapResult["snat"],
					DNat: mapResult["dnat"],
				}
			}
		}
	}

	// If we can't find either spx-ctrl or db info
	if (resp.StatusCode == http.StatusNotFound) && err != nil {
		log.Error().Str("effectiveId", productEId).Msg(consts.SpxResourceNotFound)
		httpError.Http(w, r, http.StatusNotFound).Str("eid", productEId).Msg(consts.SpxResourceNotFound)
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

// CreateEip
//
//	@Summary		Create EIP
//	@Description	Create a new EIP
//	@Tags			v1, Superphenix Controller
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path		string			true	"Organization ID"
//	@Param			az			path		string			true	"AZ Code"
//	@Param			projectId	path		string			true	"Project ID"
//	@Param			Body		body		CreateEipBody	true	"Eip info"
//	@Success		200			{object}	controller.CreateResponse
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/eip [post]
//	@Security		Bearer[OrganizationRead, ProjectEipWrite]
func (h *Service) CreateEip(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, org, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	// Create the product in DB
	var body CreateEipBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	eip, m, err := controller.CreateIntoDb(r.Context(), body.General.ProductName, model.ProductTypeEIP, azDb.Code, org.ID, projectEntity.ID)
	if err != nil {
		log.Err(err).Msg("Failed to save product into database")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	// Send request to superphenix-controller
	newBody := CreateEipSpxControllerBody{
		Metadata: m,
		General: struct {
			SubnetEId string `json:"subnetEid"`
		}{SubnetEId: body.General.SubnetEId},
		Spec: body.Spec,
	}

	// update body
	marshal, err := json.Marshal(newBody)
	if err != nil {
		log.Err(err).Msg("Failed to marshal body")
		httpError.Http(w, r, http.StatusBadRequest).Msg(http.StatusText(http.StatusBadRequest))
		return
	}
	resp, err := proxy.SendProxy(r, azDb, config.ApiPrefix, bytes.NewReader(marshal))
	if err != nil {
		log.Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		controller.WriteCreateResponse(w, eip.EffectiveID)
	} else {
		ctrlutils.CleanDb(r.Context(), eip.ID)
		ctrlutils.HandleControllerError(w, r, resp, consts.SpxResourceCreationFailureCode, consts.SpxResourceCreationFailure)
		return
	}
}

// UpdateEip
//
//	@Summary		Update EIP
//	@Description	Update an EIP
//	@Tags			v1, Superphenix Controller
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path		string			true	"Organization ID"
//	@Param			az			path		string			true	"AZ Code"
//	@Param			projectId	path		string			true	"Project ID"
//	@Param			Body		body		UpdateEipBody	true	"Eip info"
//	@Success		200			{object}	EipFullResponse	"EIP"
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/eip/{effectiveId} [post]
//	@Security		Bearer[OrganizationRead, ProjectEipWrite]
func (h *Service) UpdateEip(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, org, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	var body UpdateEipBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}
	productEid := chi.URLParam(r, "effectiveId")
	eip, err := controller.UpdateIntoDb(r.Context(), productEid, body.General.ProductName, model.ProductTypeEIP, azDb.Code, projectEntity.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
			return
		}
		log.Error().Err(err).Msg("Failed to save product in database")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}

	// Send request to superphenix-controller
	newBody := UpdateEipSpxControllerBody{
		Metadata: spxId.Metadata{
			OrgId:               org.ID.String(),
			ProjectId:           projectEntity.ID.String(),
			ResourceEffectiveId: eip.EffectiveID,
			ResourceLocalId:     eip.ID.String(),
		},
		Spec: body.Spec,
	}

	// update body
	marshal, err := json.Marshal(newBody)
	if err != nil {
		log.Err(err).Msg("Failed to marshal body")
		httpError.Http(w, r, http.StatusBadRequest).Msg(http.StatusText(http.StatusBadRequest))
		return
	}
	resp, err := proxy.SendProxy(r, azDb, config.ApiPrefix, bytes.NewReader(marshal))
	if err != nil {
		log.Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		w.WriteHeader(http.StatusOK)
	} else if resp.StatusCode == http.StatusNotFound {
		httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
		return
	} else {
		ctrlutils.HandleControllerError(w, r, resp, consts.SpxResourceUpdateFailureCode, consts.SpxResourceUpdateFailure)
		return
	}
}

// DeleteEip
//
//	@Summary		Delete EIP
//	@Description	Delete EIP by effective ID
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Eip EID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/eip/{effectiveId} [delete]
//	@Security		Bearer[OrganizationRead, ProjectEipWrite]
func (h *Service) DeleteEip(w http.ResponseWriter, r *http.Request) {
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
		log.Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
		return
	}
	defer resp.Body.Close()

	// If the product was deleted by the controller or no longer exists.
	if resp.StatusCode == 200 || resp.StatusCode == 404 {
		productEId := chi.URLParam(r, "effectiveId")
		rowsAffected, err := product.DeleteByEIdAndAZCodeAndProject(productEId, azDb.Code, projectEntity.ID)
		if err != nil {
			log.Err(err).Msg(consts.SpxResourceDeletionFailure)
			httpError.Http(w, r, consts.SpxResourceDeletionFailureCode).Msg(consts.SpxResourceDeletionFailure)
			return
		}
		if resp.StatusCode == http.StatusNotFound && rowsAffected == 0 {
			httpError.Http(w, r, http.StatusNotFound).Str("eid", productEId).Msg(consts.SpxResourceNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	} else {
		ctrlutils.HandleControllerError(w, r, resp, consts.SpxResourceDeletionFailureCode, consts.SpxResourceDeletionFailure)
		return
	}
}

// combineListResult regroup results from db and controller
func combineListResult(concatResults map[string][]interface{}, resources []model.Product, mapResourceCheck map[uuid.UUID]bool) []EipFullResponse {
	combineResults := make([]EipFullResponse, 0)
	for azCode, results := range concatResults {
		for _, result := range results {
			mapResult := result.(map[string]interface{})
			found := false
			for _, p := range resources {
				if p.ID.String() == mapResult["id"] {
					combineResults = append(combineResults, EipFullResponse{
						ProductResponse: ProductResponse{
							ID:            p.ID.String(),
							EId:           mapResult["eid"].(string),
							ProductName:   p.ProductName,
							CodeAZ:        azCode, // we use az code to handle instance under PRA
							ProductTypeId: p.ProductTypeId,
							Gitops:        mapResult["gitops"].(string),
						},
						Eip: mapResult["eip"],
					})
					mapResourceCheck[p.ID] = true
					found = true
					break
				}
			}

			// If only gitops
			if !found {
				combineResults = append(combineResults, EipFullResponse{
					ProductResponse: ProductResponse{
						ID:          mapResult["id"].(string),
						EId:         mapResult["eid"].(string),
						ProductName: mapResult["productName"].(string),
						CodeAZ:      azCode,
						Gitops:      mapResult["gitops"].(string),
					},
					Eip: mapResult["eip"],
				})
			}
		}

	}

	// Check for not found resources
	for _, p := range resources {
		if mapResourceCheck[p.ID] == false {
			combineResults = append(combineResults, EipFullResponse{
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

	slices.SortFunc(combineResults, func(a, b EipFullResponse) int {
		return controller.CompareProductResult(a.ProductResponse, b.ProductResponse)
	})

	return combineResults
}

// ProductResponse is the shared response base defined by the controller kit.
type ProductResponse = controller.ProductResponse

type CreateEipBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
		SubnetEId   string `json:"subnetEId"` // SubnetEID = Subnet Effective ID
	} `json:"general"`
	Spec struct {
		InternalIP string        `json:"internalIP,omitempty"`
		SNAT       []string      `json:"snat,omitempty"`
		DNAT       []EipDNATBody `json:"dnat,omitempty"`
	} `json:"spec"`
}

type UpdateEipBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
	} `json:"general"`
	Spec struct {
		InternalIP string        `json:"internalIP,omitempty"`
		SNAT       []string      `json:"snat,omitempty"`
		DNAT       []EipDNATBody `json:"dnat,omitempty"`
	} `json:"spec"`
}

// CreateEipSpxControllerBody is the body send to superphenix-controller to create an EIP
type CreateEipSpxControllerBody struct {
	spxId.Metadata
	General struct {
		SubnetEId string `json:"subnetEid"` // SubnetEID = Subnet Effective ID
	} `json:"general"`
	Spec struct {
		InternalIP string        `json:"internalIP,omitempty"`
		SNAT       []string      `json:"snat,omitempty"`
		DNAT       []EipDNATBody `json:"dnat,omitempty"`
	} `json:"spec"`
}

// UpdateEipSpxControllerBody is the body send to superphenix-controller to update an EIP
type UpdateEipSpxControllerBody struct {
	spxId.Metadata
	Spec struct {
		InternalIP string        `json:"internalIP,omitempty"`
		SNAT       []string      `json:"snat,omitempty"`
		DNAT       []EipDNATBody `json:"dnat,omitempty"`
	} `json:"spec"`
}

type EipDNATBody struct {
	ExternalPort string `json:"externalPort"`
	InternalIP   string `json:"internalIP"`
	InternalPort string `json:"internalPort"`
	Protocol     string `json:"protocol"`
}

type EipFullResponse struct {
	ProductResponse `json:",inline"`
	Eip             interface{} `json:"eip"`
	Fip             interface{} `json:"fip,omitempty"`
	SNat            interface{} `json:"snat,omitempty"`
	DNat            interface{} `json:"dnat,omitempty"`
}
