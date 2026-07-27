package snapshot

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

// ListSnapshots
//
//	@Summary		Retrieve all snapshots
//	@Description	Retrieve all snapshots across AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string					true	"Organization ID"
//	@Param			projectId	path	string					true	"Project ID"
//	@Success		200			{array}	SnapshotFullResponse	"Snapshots"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{projectId}/snapshot [get]
//	@Security		Bearer[OrganizationRead, ProjectSnapshotRead]
func (h *Service) ListSnapshots(w http.ResponseWriter, r *http.Request) {
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

	resourcesDb, err := product.FindAllByProjectIdAndResourceType(project.ID.String(), model.ProductTypeSnapshot)
	if err != nil {
		log.Err(err).Str("projectId", project.ID.String()).Str("resourceType", model.ProductTypeSnapshot).Msg(consts.SpxFindAllResourcesError)
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

// ListAZSnapshots
//
//	@Summary		Retrieve all AZ snapshots
//	@Description	Retrieve all snapshots for a specific AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string					true	"Organization ID"
//	@Param			az			path	string					true	"AZ Code"
//	@Param			projectId	path	string					true	"Project ID"
//	@Success		200			{array}	SnapshotFullResponse	"Snapshots"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/snapshot [get]
//	@Security		Bearer[OrganizationRead, ProjectSnapshotRead]
func (h *Service) ListAZSnapshots(w http.ResponseWriter, r *http.Request) {
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

	resources, err := product.FindAllByProjectIdAndResourceTypeAndCodeAZ(projectDb.ID.String(), model.ProductTypeSnapshot, azDb.Code)
	if err != nil {
		log.Err(err).
			Str("projectId", projectDb.ID.String()).
			Str("resourceType", model.ProductTypeSnapshot).
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

// GetSnapshot
//
//	@Summary		Get snapshot
//	@Description	Get snapshot by Effective ID
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path		string					true	"Organization ID"
//	@Param			az			path		string					true	"AZ Code"
//	@Param			projectId	path		string					true	"Project ID"
//	@Param			effectiveId	path		string					true	"Snapshot EID"
//	@Success		200			{object}	SnapshotFullResponse	"Snapshot"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/snapshot/{effectiveId} [get]
//	@Security		Bearer[OrganizationRead, ProjectSnapshotRead]
func (h *Service) GetSnapshot(w http.ResponseWriter, r *http.Request) {
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
		// If the request is not OK or NotFound then log it
		log.Error().Str("path", r.URL.Path).Str("status", resp.Status).Int("statusCode", resp.StatusCode).Str("az", azDb.Code).Msg("Request on superphenix-controller failed")
	}
	defer resp.Body.Close()

	resourceEId := chi.URLParam(r, "effectiveId")
	dbProduct, dbErr := product.FindByEId(resourceEId)
	var result SnapshotFullResponse

	// If we didn't find spx-ctrl, but we got db info
	if (resp.StatusCode != http.StatusOK) && dbErr == nil {
		result = SnapshotFullResponse{
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

			result = SnapshotFullResponse{
				ProductResponse: ProductResponse{
					ID:          mapResult["id"].(string),
					EId:         mapResult["eid"].(string),
					ProductName: mapResult["productName"].(string),
					CodeAZ:      azDb.Code,
					Gitops:      mapResult["gitops"].(string),
				},
				Snapshot:         mapResult["snapshot"],
				SnapshotSchedule: mapResult["snapshotSchedule"],
			}
		} else {
			//	We got both db and spx-ctrl info
			if dbProduct.ID.String() == mapResult["id"] {
				result = SnapshotFullResponse{
					ProductResponse: ProductResponse{
						ID:            dbProduct.ID.String(),
						EId:           mapResult["eid"].(string),
						ProductName:   dbProduct.ProductName,
						CodeAZ:        azDb.Code,
						ProductTypeId: dbProduct.ProductTypeId,
						Gitops:        mapResult["gitops"].(string),
					},
					Snapshot:         mapResult["snapshot"],
					SnapshotSchedule: mapResult["snapshotSchedule"],
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
	marshal, dbErr := json.Marshal(result)
	if dbErr != nil {
		log.Err(err).Msg(consts.SpxResponseParseFailure)
		httpError.Http(w, r, consts.SpxResponseParseFailureCode).Msg(consts.SpxResponseParseFailure)
		return
	}
	_, _ = w.Write(marshal)
}

// CreateSnapshot
//
//	@Summary		Create snapshot
//	@Description	Create a new snapshot and start it
//	@Tags			v1, Superphenix Controller
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path		string				true	"Organization ID"
//	@Param			az			path		string				true	"AZ Code"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			Body		body		CreateSnapshotBody	true	"Snapshot info"
//	@Success		200			{object}	controller.CreateResponse
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/snapshot [post]
//	@Security		Bearer[OrganizationRead, ProjectSnapshotWrite]
func (h *Service) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, org, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	// Create the product in DB
	var body CreateSnapshotBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	snapshot, m, err := controller.CreateIntoDb(r.Context(), body.General.ProductName, model.ProductTypeSnapshot, azDb.Code, org.ID, projectEntity.ID)
	if err != nil {
		log.Err(err).Msg("Failed to save product into database")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	// Send request to superphenix-controller
	newBody := CreateSnapshotSpxControllerBody{
		Metadata: m,
		Spec: struct {
			Source        string          `json:"source"`
			Scheduled     bool            `json:"scheduled,omitempty"`
			Schedule      int             `json:"schedule,omitempty"`
			Paused        bool            `json:"paused,omitempty"`
			LabelSelector []string        `json:"labelSelector,omitempty"`
			Retention     RetentionPolicy `json:"retention,omitempty"`
		}{
			Source:        body.Spec.Source,
			Scheduled:     body.Spec.Scheduled,
			Schedule:      body.Spec.Schedule,
			Paused:        body.Spec.Paused,
			LabelSelector: body.Spec.LabelSelector,
			Retention:     body.Spec.Retention,
		},
		SnapshotSchedule: SnapshotScheduleConfig{
			MinHour: h.cfg.ProductsConfig.SnapshotSchedule.MinHour,
			MaxHour: h.cfg.ProductsConfig.SnapshotSchedule.MaxHour,
		},
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
		controller.WriteCreateResponse(w, snapshot.EffectiveID)
	} else {
		ctrlutils.CleanDb(r.Context(), snapshot.ID)
		ctrlutils.HandleControllerError(w, r, resp, consts.SpxResourceCreationFailureCode, consts.SpxResourceCreationFailure)
		return
	}
}

// UpdateSnapshot
//
//	@Summary		Update snapshot schedule
//	@Description	Update a snapshot schedule by effective ID
//	@Tags			v1, SPX Controller
//	@Produce		json
//	@Param			orgaId		path	string				true	"Organization ID"
//	@Param			az			path	string				true	"AZ Code"
//	@Param			projectId	path	string				true	"Project ID"
//	@Param			effectiveId	path	string				true	"Snapshot EID"
//	@Param			Body		body	UpdateSnapshotBody	true	"Snapshot Schedule info"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/snapshot/{effectiveId} [post]
//	@Security		Bearer[OrganizationRead, ProjectSnapshotWrite]
func (h *Service) UpdateSnapshot(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, _, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	var body UpdateSnapshotBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	productEid := chi.URLParam(r, "effectiveId")
	if _, err := controller.UpdateIntoDb(r.Context(), productEid, body.General.ProductName, model.ProductTypeSnapshot, azDb.Code, projectEntity.ID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
			return
		}
		log.Error().Err(err).Msg("Failed to save product in database")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}

	// Send request to az-controller
	newBody := UpdateSnapshotAzControllerBody{
		Spec: struct {
			Schedule      int             `json:"schedule"`
			Paused        bool            `json:"paused"`
			LabelSelector []string        `json:"labelSelector,omitempty"`
			Retention     RetentionPolicy `json:"retention"`
		}{
			Schedule:      body.Spec.Schedule,
			Paused:        body.Spec.Paused,
			LabelSelector: body.Spec.LabelSelector,
			Retention:     body.Spec.Retention,
		},
		SnapshotSchedule: SnapshotScheduleConfig{
			MinHour: h.cfg.ProductsConfig.SnapshotSchedule.MinHour,
			MaxHour: h.cfg.ProductsConfig.SnapshotSchedule.MaxHour,
		},
	}

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

// DeleteSnapshot
//
//	@Summary		Delete snapshot
//	@Description	Delete snapshot by effective ID
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Snapshot EID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/snapshot/{effectiveId} [delete]
//	@Security		Bearer[OrganizationRead, ProjectSnapshotWrite]
func (h *Service) DeleteSnapshot(w http.ResponseWriter, r *http.Request) {
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
func combineListResult(concatResults map[string][]interface{}, resources []model.Product, mapResourceCheck map[uuid.UUID]bool) []SnapshotFullResponse {
	combineResults := make([]SnapshotFullResponse, 0)
	for azCode, results := range concatResults {
		for _, result := range results {
			mapResult := result.(map[string]interface{})
			found := false
			for _, p := range resources {
				if p.ID.String() == mapResult["id"] {
					combineResults = append(combineResults, SnapshotFullResponse{
						ProductResponse: ProductResponse{
							ID:            p.ID.String(),
							EId:           mapResult["eid"].(string),
							ProductName:   p.ProductName,
							CodeAZ:        azCode, // we use az code to handle instance under PRA
							ProductTypeId: p.ProductTypeId,
							Gitops:        mapResult["gitops"].(string),
						},
						Snapshot:         mapResult["snapshot"],
						SnapshotSchedule: mapResult["snapshotSchedule"],
					})
					mapResourceCheck[p.ID] = true
					found = true
					break
				}
			}

			// If only gitops
			if !found {
				combineResults = append(combineResults, SnapshotFullResponse{
					ProductResponse: ProductResponse{
						ID:          mapResult["id"].(string),
						EId:         mapResult["eid"].(string),
						ProductName: mapResult["productName"].(string),
						CodeAZ:      azCode,
						Gitops:      mapResult["gitops"].(string),
					},
					Snapshot:         mapResult["snapshot"],
					SnapshotSchedule: mapResult["snapshotSchedule"],
				})
			}
		}

	}

	// Check for not found resources
	for _, p := range resources {
		if mapResourceCheck[p.ID] == false {
			combineResults = append(combineResults, SnapshotFullResponse{
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

	slices.SortFunc(combineResults, func(a, b SnapshotFullResponse) int {
		return controller.CompareProductResult(a.ProductResponse, b.ProductResponse)
	})

	return combineResults
}

// ProductResponse is the shared response base defined by the controller kit.
type ProductResponse = controller.ProductResponse

type CreateSnapshotBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
	} `json:"general"`
	Spec struct {
		Source        string          `json:"source,omitempty"` // Source EID
		Scheduled     bool            `json:"scheduled,omitempty"`
		Schedule      int             `json:"schedule,omitempty"`
		Paused        bool            `json:"paused,omitempty"`
		LabelSelector []string        `json:"labelSelector,omitempty"`
		Retention     RetentionPolicy `json:"retention,omitempty"`
	} `json:"spec"`
}

// CreateSnapshotSpxControllerBody is the body send to superphenix-controller to create a Snapshot
type CreateSnapshotSpxControllerBody struct {
	spxId.Metadata
	Spec struct {
		Source        string          `json:"source"` // Source EID
		Scheduled     bool            `json:"scheduled,omitempty"`
		Schedule      int             `json:"schedule,omitempty"`
		Paused        bool            `json:"paused,omitempty"`
		LabelSelector []string        `json:"labelSelector,omitempty"`
		Retention     RetentionPolicy `json:"retention,omitempty"`
	} `json:"spec"`
	SnapshotSchedule SnapshotScheduleConfig `json:"snapshotSchedule"`
}

type UpdateSnapshotBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
	} `json:"general"`
	Spec struct {
		Schedule      int             `json:"schedule"`
		Paused        bool            `json:"paused"`
		LabelSelector []string        `json:"labelSelector,omitempty"`
		Retention     RetentionPolicy `json:"retention"`
	} `json:"spec"`
}

// UpdateSnapshotAzControllerBody is the body send to az-controller to update a Snapshot
type UpdateSnapshotAzControllerBody struct {
	Spec struct {
		Schedule      int             `json:"schedule"`
		Paused        bool            `json:"paused"`
		LabelSelector []string        `json:"labelSelector,omitempty"`
		Retention     RetentionPolicy `json:"retention"`
	} `json:"spec"`
	SnapshotSchedule SnapshotScheduleConfig `json:"snapshotSchedule"`
}

type RetentionPolicy struct {
	//Retention time of the backups in hours (max is 960, or 40 days)
	ExpiryTime int `yaml:"expiryTime,omitempty"`
}

type SnapshotScheduleConfig struct {
	MinHour int `json:"minHour"`
	MaxHour int `json:"maxHour"`
}

type SnapshotFullResponse struct {
	ProductResponse  `json:",inline"`
	Snapshot         interface{} `json:"snapshot,omitempty"`
	SnapshotSchedule interface{} `json:"snapshotSchedule,omitempty"`
}
