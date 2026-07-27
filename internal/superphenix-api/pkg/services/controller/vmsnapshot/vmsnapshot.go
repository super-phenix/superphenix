package vmsnapshot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/az"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/product"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/proxy"
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

// ListVmSnapshots
//
//	@Summary		Retrieve all snapshots
//	@Description	Retrieve all snapshots across AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string					true	"Organization ID"
//	@Param			projectId	path	string					true	"Project ID"
//	@Success		200			{array}	VmSnapshotFullResponse	"VmSnapshots"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{projectId}/instance-snapshot [get]
//	@Security		Bearer[OrganizationRead, ProjectVmSnapshotRead]
func (h *Service) ListVmSnapshots(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orga, project, code, errMsg := ctrlutils.CheckListPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	urls := az.FindAll(orga.ID.String())

	responses, err := proxy.SendBatchProxy(r, urls, h.cfg.Controller.ApiPrefix)
	if err != nil {
		log.Err(err).Msg(consts.SpxProxyToAZFailure)
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Msg(consts.SpxProxyToAZFailure)
		return
	}
	concatResults := ctrlutils.ConcatResponses(r.Context(), responses)

	resourcesDb, err := product.FindAllByProjectIdAndResourceType(project.ID.String(), model.ProductTypeVmSnapshot)
	if err != nil {
		log.Err(err).Str("projectId", project.ID.String()).Str("resourceType", model.ProductTypeVmSnapshot).Msg(consts.SpxFindAllResourcesError)
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

// ListAZVmSnapshots
//
//	@Summary		Retrieve all AZ snapshots
//	@Description	Retrieve all snapshots for a specific AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string					true	"Organization ID"
//	@Param			az			path	string					true	"AZ Code"
//	@Param			projectId	path	string					true	"Project ID"
//	@Success		200			{array}	VmSnapshotFullResponse	"VmSnapshots"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance-snapshot [get]
//	@Security		Bearer[OrganizationRead, ProjectVmSnapshotRead]
func (h *Service) ListAZVmSnapshots(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, _, projectDb, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	resp, err := proxy.SendProxy(r, azDb, h.cfg.Controller.ApiPrefix, http.NoBody)
	if err != nil {
		log.Error().Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		log.Error().Str("path", r.URL.Path).Str("status", resp.Status).Int("statusCode", resp.StatusCode).Str("az", azDb.Code).Msg("Request on superphenix-controller failed")
	}
	concatResults := ctrlutils.ConcatResponses(r.Context(), map[string]*http.Response{azDb.Code: resp})

	resources, err := product.FindAllByProjectIdAndResourceTypeAndCodeAZ(projectDb.ID.String(), model.ProductTypeVmSnapshot, azDb.Code)
	if err != nil {
		log.Err(err).
			Str("projectId", projectDb.ID.String()).
			Str("resourceType", model.ProductTypeVmSnapshot).
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

// GetVmSnapshot
//
//	@Summary		Get snapshot
//	@Description	Get snapshot by Effective ID
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path		string					true	"Organization ID"
//	@Param			az			path		string					true	"AZ Code"
//	@Param			projectId	path		string					true	"Project ID"
//	@Param			effectiveId	path		string					true	"VmSnapshot EID"
//	@Success		200			{object}	VmSnapshotFullResponse	"VmSnapshot"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance-snapshot/{effectiveId} [get]
//	@Security		Bearer[OrganizationRead, ProjectVmSnapshotRead]
func (h *Service) GetVmSnapshot(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, _, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	if !ctrlutils.CheckProductBelongsToProject(w, r, projectEntity.ID, azDb.Code) {
		return
	}

	resp, err := proxy.SendProxy(r, azDb, h.cfg.Controller.ApiPrefix, http.NoBody)
	if err != nil {
		log.Error().Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		// If the request is not OK or NotFound then log it
		log.Error().Str("path", r.URL.Path).Str("status", resp.Status).Int("statusCode", resp.StatusCode).Str("az", azDb.Code).Msg("Request on superphenix-controller failed")
	}
	defer resp.Body.Close()

	resourceEId := chi.URLParam(r, "effectiveId")
	dbProduct, dbErr := product.FindByEId(resourceEId)
	var result VmSnapshotFullResponse

	// If we didn't find spx-ctrl, but we got db info
	if (resp.StatusCode != http.StatusOK) && dbErr == nil {
		result = VmSnapshotFullResponse{
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

			result = VmSnapshotFullResponse{
				ProductResponse: ProductResponse{
					ID:          mapResult["id"].(string),
					EId:         mapResult["eid"].(string),
					ProductName: mapResult["productName"].(string),
					CodeAZ:      azDb.Code,
					Gitops:      mapResult["gitops"].(string),
				},
				VmSnapshot:        mapResult["vmSnapshot"],
				VmSnapshotContent: mapResult["vmSnapshotContent"],
			}
		} else {
			//	We got both db and spx-ctrl info
			if dbProduct.ID.String() == mapResult["id"] {
				result = VmSnapshotFullResponse{
					ProductResponse: ProductResponse{
						ID:            dbProduct.ID.String(),
						EId:           mapResult["eid"].(string),
						ProductName:   dbProduct.ProductName,
						CodeAZ:        azDb.Code,
						ProductTypeId: dbProduct.ProductTypeId,
						Gitops:        mapResult["gitops"].(string),
					},
					VmSnapshot:        mapResult["vmSnapshot"],
					VmSnapshotContent: mapResult["vmSnapshotContent"],
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

// CreateVmSnapshot
//
//	@Summary		Create snapshot
//	@Description	Create a new snapshot and start it
//	@Tags			v1, Superphenix Controller
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path		string					true	"Organization ID"
//	@Param			az			path		string					true	"AZ Code"
//	@Param			projectId	path		string					true	"Project ID"
//	@Param			Body		body		CreateVmSnapshotBody	true	"VmSnapshot info"
//	@Success		200			{object}	controller.CreateResponse
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance-snapshot [post]
//	@Security		Bearer[OrganizationRead, ProjectVmSnapshotWrite]
func (h *Service) CreateVmSnapshot(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, org, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	// Create the product in DB
	var body CreateVmSnapshotBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	snapshot, m, err := controller.CreateIntoDb(r.Context(), body.General.ProductName, model.ProductTypeVmSnapshot, azDb.Code, org.ID, projectEntity.ID)
	if err != nil {
		log.Err(err).Msg("Failed to save product into database")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	// Send request to superphenix-controller
	newBody := CreateVmSnapshotSpxControllerBody{
		Metadata: m,
		General: struct {
			Source string `json:"source"`
		}{
			Source: body.General.Source,
		},
	}

	// update body
	marshal, err := json.Marshal(newBody)
	if err != nil {
		log.Err(err).Msg("Failed to marshal body")
		httpError.Http(w, r, http.StatusBadRequest).Msg(http.StatusText(http.StatusBadRequest))
		return
	}
	resp, err := proxy.SendProxy(r, azDb, h.cfg.Controller.ApiPrefix, bytes.NewReader(marshal))
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

// DeleteVmSnapshot
//
//	@Summary		Delete snapshot
//	@Description	Delete snapshot by effective ID
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"VmSnapshot EID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance-snapshot/{effectiveId} [delete]
//	@Security		Bearer[OrganizationRead, ProjectVmSnapshotWrite]
func (h *Service) DeleteVmSnapshot(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	azDb, _, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	if !ctrlutils.CheckProductBelongsToProject(w, r, projectEntity.ID, azDb.Code) {
		return
	}

	resp, err := proxy.SendProxy(r, azDb, h.cfg.Controller.ApiPrefix, http.NoBody)
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

// RestoreVmSnapshot
//
//	@Summary		Restore a VM snapshot
//	@Description	Restore a VM snapshot by creating a new instance from it
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Snapshot EID"
//	@Param			name		query	string	true	"New instance name"
//	@Param			localId		query	string	true	"New instance local ID (UUID)"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance-snapshot/{effectiveId}/restore [get]
//	@Security		Bearer[OrganizationRead, ProjectSnapshotWrite, ProjectInstanceWrite]
func (h *Service) RestoreVmSnapshot(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, org, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	// Check query params
	name := r.URL.Query().Get("name")
	// Check if the name complies with the conditions
	if len(name) > 63 {
		reason := fmt.Errorf("name too long %d > 63", len(name))
		log.Err(reason).Str("name", name).Msg(consts.SpxResourceCreationFailure)
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Str("reason", reason.Error()).Msg(consts.SpxResourceCreationFailure)
		return
	}

	// Check localId
	localId := r.URL.Query().Get("localId")
	localIdUuid, err := uuid.Parse(localId)
	if err != nil {
		log.Err(err).Str("localId", localId).Msg("Failed to parse localId")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}
	m := spxId.Metadata{}
	if err = m.GenerateMetadata(projectEntity.ID.String(), org.ID.String(), localId); err != nil {
		log.Err(err).Str("localId", localId).Msg("Failed to generate metadata")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	// TODO check if the localID provided is the good one (HOW ????)
	//resourceEId := chi.URLParam(r, "effectiveId")
	//if m.GetResourceEffectiveID() != resourceEId {
	//	httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Err(err).Str("effectiveId", resourceEId).Str("computeEid", m.GetResourceEffectiveID()).Msg(consts.SpxResourceCreationFailure)
	//	return
	//
	//}

	resp, err := proxy.SendProxy(r, azDb, h.cfg.Controller.ApiPrefix, http.NoBody)
	if err != nil {
		log.Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 404 {
		// Check if instance exist in db
		var instance model.Product
		result := db.Client.Unscoped().Where(model.Product{EffectiveID: m.GetResourceEffectiveID()}).First(&instance)

		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			log.Err(result.Error).Str("effectiveId", m.GetResourceEffectiveID()).Msg("Failed to find product in database")
			httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
			return
		}

		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			log.Debug().Msg("Instance not found, creation in db")
			instance.ID = localIdUuid
			instance.ProductName = name
			instance.CodeAZ = azDb.Code
			instance.ProjectId = projectEntity.ID
			instance.ProductTypeId = model.ProductTypeInstance
			instance.EffectiveID = m.GetResourceEffectiveID()

			_, err := product.Save(instance)
			if err != nil {
				log.Err(err).Msg("Failed to save product in database")
				httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
				return
			}
		} else {
			log.Debug().Msgf("Instance found, deletion status %t", instance.DeletedAt.Valid)
			result = db.Client.Unscoped().Model(instance).Updates(map[string]interface{}{"deleted_at": nil, "product_name": name})
			if result.Error != nil {
				log.Debug().Err(result.Error).Msg("failed to restore instance in db")
			}
		}

	} else {
		ctrlutils.HandleControllerError(w, r, resp, consts.SpxResourceCreationFailureCode, consts.SpxResourceCreationFailure)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// CloneVmSnapshot
//
//	@Summary		Clone a VM snapshot
//	@Description	Clone a VM snapshot by creating a new instance from it
//	@Tags			v1, SPX Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Snapshot EID"
//	@Param			name		query	string	true	"New instance name"
//	@Success		201
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance-snapshot/{effectiveId}/clone [get]
//	@Security		Bearer[OrganizationRead, ProjectSnapshotWrite, ProjectInstanceWrite]
func (h *Service) CloneVmSnapshot(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, org, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	// Create the product in DB
	var body CloneVmSnapshotBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	instance, _, err := controller.CreateIntoDb(r.Context(), body.Name, model.ProductTypeInstance, azDb.Code, org.ID, projectEntity.ID)
	if err != nil {
		log.Err(err).Msg("Failed to save product into database")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	newBody := CloneVmSnapshotAZControllerBody{
		Id: instance.ID.String(),
	}

	// update body
	marshal, err := json.Marshal(newBody)
	if err != nil {
		log.Err(err).Msg("Failed to marshal body")
		httpError.Http(w, r, http.StatusBadRequest).Msg(http.StatusText(http.StatusBadRequest))
		return
	}
	resp, err := proxy.SendProxy(r, azDb, h.cfg.Controller.ApiPrefix, bytes.NewReader(marshal))
	if err != nil {
		log.Err(err).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
		httpError.Http(w, r, consts.SpxProxyToAZFailureCode).Str("az", azDb.Code).Msg(consts.SpxProxyToAZFailure)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		var cloneResp CloneVmSnapshotAZControllerResponse
		if err := json.NewDecoder(resp.Body).Decode(&cloneResp); err != nil {
			log.Err(err).Msg("Failed to decode clone response")
			httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
			return
		}

		var diskErrors []error
		for i, disk := range cloneResp.Disks {
			diskId, err := uuid.Parse(disk.Id)
			if err != nil {
				log.Err(err).Str("diskId", disk.Id).Msg("Failed to parse disk ID")
				diskErrors = append(diskErrors, err)
				continue
			}

			diskName := fmt.Sprintf("%s-disk-%d", body.Name, i)
			_, err = product.Save(model.Product{
				Model:         model.Model{ID: diskId},
				ProductName:   diskName,
				CodeAZ:        azDb.Code,
				ProjectId:     projectEntity.ID,
				ProductTypeId: model.ProductTypeDisk,
				EffectiveID:   disk.Eid,
			})
			if err != nil {
				log.Err(err).Str("diskName", diskName).Msg("Failed to save disk into database")
				diskErrors = append(diskErrors, err)
				continue
			}
		}
		if len(diskErrors) > 0 {
			log.Error().Errs("errors", diskErrors).Msg("Errors occurred while saving disks")
			httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
			return
		}
	} else {
		ctrlutils.CleanDb(r.Context(), instance.ID)
		ctrlutils.HandleControllerError(w, r, resp, consts.SpxResourceCreationFailureCode, consts.SpxResourceCreationFailure)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// combineListResult regroup results from db and controller
func combineListResult(concatResults map[string][]interface{}, resources []model.Product, mapResourceCheck map[uuid.UUID]bool) []VmSnapshotFullResponse {
	combineResults := make([]VmSnapshotFullResponse, 0)
	for azCode, results := range concatResults {
		for _, result := range results {
			mapResult := result.(map[string]interface{})
			found := false
			for _, p := range resources {
				if p.ID.String() == mapResult["id"] {
					combineResults = append(combineResults, VmSnapshotFullResponse{
						ProductResponse: ProductResponse{
							ID:            p.ID.String(),
							EId:           mapResult["eid"].(string),
							ProductName:   p.ProductName,
							CodeAZ:        azCode, // we use az code to handle instance under PRA
							ProductTypeId: p.ProductTypeId,
							Gitops:        mapResult["gitops"].(string),
						},
						VmSnapshot:        mapResult["vmSnapshot"],
						VmSnapshotContent: mapResult["vmSnapshotContent"],
					})
					mapResourceCheck[p.ID] = true
					found = true
					break
				}
			}

			// If only gitops
			if !found {
				combineResults = append(combineResults, VmSnapshotFullResponse{
					ProductResponse: ProductResponse{
						ID:          mapResult["id"].(string),
						EId:         mapResult["eid"].(string),
						ProductName: mapResult["productName"].(string),
						CodeAZ:      azCode,
						Gitops:      mapResult["gitops"].(string),
					},
					VmSnapshot:        mapResult["vmSnapshot"],
					VmSnapshotContent: mapResult["vmSnapshotContent"],
				})
			}
		}

	}

	// Check for not found resources
	for _, p := range resources {
		if mapResourceCheck[p.ID] == false {
			combineResults = append(combineResults, VmSnapshotFullResponse{
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

	slices.SortFunc(combineResults, func(a, b VmSnapshotFullResponse) int {
		return controller.CompareProductResult(a.ProductResponse, b.ProductResponse)
	})

	return combineResults
}

// ProductResponse is the shared response base defined by the controller kit.
type ProductResponse = controller.ProductResponse

type CreateVmSnapshotBody struct {
	General struct {
		ProductName string `json:"productName" validate:"max=63"`
		Source      string `json:"source"` // Source EID
	} `json:"general"`
}

// CreateVmSnapshotSpxControllerBody is the body send to superphenix-controller to create a VmSnapshot
type CreateVmSnapshotSpxControllerBody struct {
	spxId.Metadata
	General struct {
		Source string `json:"source"` // Source EID
	} `json:"general"`
}

type CloneVmSnapshotBody struct {
	Name string `json:"name" validate:"max=63"`
}

type CloneVmSnapshotAZControllerBody struct {
	Id string `json:"id"`
}

type CloneVmSnapshotAZControllerResponse struct {
	Disks []struct {
		Id  string `json:"id"`
		Eid string `json:"eid"`
	} `json:"disks"`
}

type VmSnapshotFullResponse struct {
	ProductResponse   `json:",inline"`
	VmSnapshot        interface{} `json:"vmSnapshot"`
	VmSnapshotContent interface{} `json:"vmSnapshotContent"`
}
