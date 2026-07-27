package firewall

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

// ListFirewalls
//
//	@Summary		Retrieve all firewalls
//	@Description	Retrieve all firewalls across AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string					true	"Organization ID"
//	@Param			projectId	path	string					true	"Project ID"
//	@Success		200			{array}	FirewallFullResponse	"Firewalls"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{projectId}/firewall [get]
//	@Security		Bearer[OrganizationRead, ProjectFirewallRead]
func (h *Service) ListFirewalls(w http.ResponseWriter, r *http.Request) {
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

	resourcesDb, err := product.FindAllByProjectIdAndResourceType(project.ID.String(), model.ProductTypeFirewall)
	if err != nil {
		log.Err(err).Str("projectId", project.ID.String()).Str("resourceType", model.ProductTypeFirewall).Msg(consts.SpxFindAllResourcesError)
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

// ListAZFirewalls
//
//	@Summary		Retrieve all AZ firewalls
//	@Description	Retrieve all firewalls for a specific AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string					true	"Organization ID"
//	@Param			az			path	string					true	"AZ Code"
//	@Param			projectId	path	string					true	"Project ID"
//	@Success		200			{array}	FirewallFullResponse	"Firewalls"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/firewall [get]
//	@Security		Bearer[OrganizationRead, ProjectFirewallRead]
func (h *Service) ListAZFirewalls(w http.ResponseWriter, r *http.Request) {
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

	resources, err := product.FindAllByProjectIdAndResourceTypeAndCodeAZ(projectDb.ID.String(), model.ProductTypeFirewall, azDb.Code)
	if err != nil {
		log.Err(err).
			Str("projectId", projectDb.ID.String()).
			Str("resourceType", model.ProductTypeFirewall).
			Msg(consts.SpxFindAllResourcesError)
		httpError.Http(w, r, consts.SpxFindAllResourcesErrorCode).Msg(consts.SpxFindAllResourcesError)
		return
	}

	mapResourceCheck := make(map[uuid.UUID]bool)
	for _, p := range resources {
		mapResourceCheck[p.ID] = false
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

// GetFirewall
//
//	@Summary		Get firewall
//	@Description	Get firewall by Effective ID
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path		string					true	"Organization ID"
//	@Param			az			path		string					true	"AZ Code"
//	@Param			projectId	path		string					true	"Project ID"
//	@Param			effectiveId	path		string					true	"Firewall EID"
//	@Success		200			{object}	FirewallFullResponse	"Firewall"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/firewall/{effectiveId} [get]
//	@Security		Bearer[OrganizationRead, ProjectFirewallRead]
func (h *Service) GetFirewall(w http.ResponseWriter, r *http.Request) {
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
	dbProduct, err := product.FindByEId(resourceEId)
	var result FirewallFullResponse

	if (resp.StatusCode != http.StatusOK) && err == nil {
		result = FirewallFullResponse{
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
			log.Info().Ctx(r.Context()).Err(err).Str("resourceEId", resourceEId).Msg("Resource not found in DB")

			result = FirewallFullResponse{
				ProductResponse: ProductResponse{
					ID:          mapResult["id"].(string),
					EId:         mapResult["eid"].(string),
					ProductName: mapResult["productName"].(string),
					CodeAZ:      azDb.Code,
					Gitops:      mapResult["gitops"].(string),
				},
				Firewall: mapResult["firewall"],
			}
		} else {
			//	We got both db and spx-ctrl info
			if dbProduct.ID.String() == mapResult["id"] {
				result = FirewallFullResponse{
					ProductResponse: ProductResponse{
						ID:            dbProduct.ID.String(),
						EId:           mapResult["eid"].(string),
						ProductName:   dbProduct.ProductName,
						CodeAZ:        azDb.Code,
						ProductTypeId: dbProduct.ProductTypeId,
						Gitops:        mapResult["gitops"].(string),
					},
					Firewall: mapResult["firewall"],
				}
			}
		}
	}

	// If we can't find either spx-ctrl or db info
	if (resp.StatusCode == http.StatusNotFound) && err != nil {
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

// CreateFirewall
//
//	@Summary		Create firewall
//	@Description	Create a new firewall
//	@Tags			v1, Superphenix Controller
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path		string				true	"Organization ID"
//	@Param			az			path		string				true	"AZ Code"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			Body		body		CreateFirewallBody	true	"Firewall info"
//	@Success		200			{object}	controller.CreateResponse
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/firewall [post]
//	@Security		Bearer[OrganizationRead, ProjectFirewallWrite]
func (h *Service) CreateFirewall(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, org, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	// Create the product in DB
	var body CreateFirewallBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	firewall, m, err := controller.CreateIntoDb(r.Context(), body.General.ProductName, model.ProductTypeFirewall, azDb.Code, org.ID, projectEntity.ID)
	if err != nil {
		log.Err(err).Msg("Failed to save product into database")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	// Send request to superphenix-controller
	newBody := CreateFirewallSpxControllerBody{
		Metadata:    m,
		Description: body.General.Description,
		Target:      body.Spec.Target,
		Ingress:     body.Spec.Ingress,
		Egress:      body.Spec.Egress,
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
		controller.WriteCreateResponse(w, firewall.EffectiveID)
	} else {
		ctrlutils.CleanDb(r.Context(), firewall.ID)
		ctrlutils.HandleControllerError(w, r, resp, consts.SpxResourceCreationFailureCode, consts.SpxResourceCreationFailure)
		return
	}
}

// UpdateFirewall
//
//	@Summary		Update firewall
//	@Description	Update a firewall
//	@Tags			v1, Superphenix Controller
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path	string				true	"Organization ID"
//	@Param			az			path	string				true	"AZ Code"
//	@Param			projectId	path	string				true	"Project ID"
//	@Param			Body		body	UpdateFirewallBody	true	"Firewall info"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/firewall/{effectiveId} [post]
//	@Security		Bearer[OrganizationRead, ProjectFirewallWrite]
func (h *Service) UpdateFirewall(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, _, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	// Update the product in DB
	var body UpdateFirewallBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	productEid := chi.URLParam(r, "effectiveId")
	if _, err := controller.UpdateIntoDb(r.Context(), productEid, body.General.ProductName, model.ProductTypeFirewall, azDb.Code, projectEntity.ID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
			return
		}
		log.Error().Err(err).Msg("Failed to save product in database")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}

	// Send request to superphenix-controller
	newBody := UpdateFirewallSpxControllerBody{
		Description: body.General.Description,
		Target:      body.Spec.Target,
		Ingress:     body.Spec.Ingress,
		Egress:      body.Spec.Egress,
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

// DeleteFirewall
//
//	@Summary		Delete firewall
//	@Description	Delete firewall by effective ID
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Firewall EID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/firewall/{effectiveId} [delete]
//	@Security		Bearer[OrganizationRead, ProjectFirewallWrite]
func (h *Service) DeleteFirewall(w http.ResponseWriter, r *http.Request) {
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
func combineListResult(concatResults map[string][]interface{}, resources []model.Product, mapResourceCheck map[uuid.UUID]bool) []FirewallFullResponse {
	combineResults := make([]FirewallFullResponse, 0)
	for azCode, results := range concatResults {
		for _, result := range results {
			mapResult := result.(map[string]interface{})
			found := false
			for _, p := range resources {
				if p.ID.String() == mapResult["id"] {
					combineResults = append(combineResults, FirewallFullResponse{
						ProductResponse: ProductResponse{
							ID:            p.ID.String(),             // Local ID
							EId:           mapResult["eid"].(string), // Effective (name k8s)
							ProductName:   p.ProductName,             // Name db - User friendly
							CodeAZ:        azCode,                    // we use az code to handle instance under PRA
							ProductTypeId: p.ProductTypeId,
							Gitops:        mapResult["gitops"].(string),
						},
						Firewall: mapResult["firewall"],
					})
					mapResourceCheck[p.ID] = true
					found = true
					break
				}
			}

			// If only gitops
			if !found {
				combineResults = append(combineResults, FirewallFullResponse{
					ProductResponse: ProductResponse{
						ID:          mapResult["id"].(string),
						EId:         mapResult["eid"].(string),
						ProductName: mapResult["productName"].(string),
						CodeAZ:      azCode,
						Gitops:      mapResult["gitops"].(string),
					},
					Firewall: mapResult["firewall"],
				})
			}
		}

	}

	// Check for not found resources
	for _, p := range resources {
		if mapResourceCheck[p.ID] == false {
			combineResults = append(combineResults, FirewallFullResponse{
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

	slices.SortFunc(combineResults, func(a, b FirewallFullResponse) int {
		return controller.CompareProductResult(a.ProductResponse, b.ProductResponse)
	})

	return combineResults
}

// ProductResponse is the shared response base defined by the controller kit.
type ProductResponse = controller.ProductResponse

type CreateFirewallBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
		Description string `json:"description"`
	} `json:"general"`
	Spec struct {
		Target  LabelSelector `json:"target"`
		Ingress []IngressRule `json:"ingress"`
		Egress  []EgressRule  `json:"egress"`
	} `json:"spec"`
}

type UpdateFirewallBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
		Description string `json:"description"`
	} `json:"general"`
	Spec struct {
		Target  LabelSelector `json:"target"`
		Ingress []IngressRule `json:"ingress"`
		Egress  []EgressRule  `json:"egress"`
	} `json:"spec"`
}

type MatchLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type MatchExpression struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

type LabelSelector struct {
	MatchLabels      []MatchLabel      `json:"matchLabels"`
	MatchExpressions []MatchExpression `json:"matchExpressions"`
}

type IPBlock struct {
	CIDR   string   `json:"CIDR"`
	Except []string `json:"Except"`
}

type Peer struct {
	PodSelector LabelSelector `json:"podSelector"`
	IPBlock     IPBlock       `json:"IPBlock"`
}
type FwPort struct {
	Port     int32  `json:"port"`
	EndPort  int32  `json:"endPort"`
	Protocol string `json:"protocol"`
}

type IngressRule struct {
	Ports    []FwPort `json:"ports"`
	From     []Peer   `json:"from"`
	AllowAll bool     `json:"allowAll"`
	DenyAll  bool     `json:"denyAll"`
}
type EgressRule struct {
	Ports    []FwPort `json:"ports"`
	To       []Peer   `json:"to"`
	AllowAll bool     `json:"allowAll"`
	DenyAll  bool     `json:"denyAll"`
}

// CreateFirewallSpxControllerBody is the body send to superphenix-controller to create a Load Balancer
type CreateFirewallSpxControllerBody struct {
	spxId.Metadata
	Description string        `json:"description"`
	Target      LabelSelector `json:"target"`
	Ingress     []IngressRule `json:"ingress"`
	Egress      []EgressRule  `json:"egress"`
}

// UpdateFirewallSpxControllerBody is the body send to superphenix-controller to update a Load Balancer
type UpdateFirewallSpxControllerBody struct {
	Description string        `json:"description"`
	Target      LabelSelector `json:"target"`
	Ingress     []IngressRule `json:"ingress"`
	Egress      []EgressRule  `json:"egress"`
}

type FirewallFullResponse struct {
	ProductResponse `json:",inline"`
	Firewall        interface{} `json:"firewall"`
}
