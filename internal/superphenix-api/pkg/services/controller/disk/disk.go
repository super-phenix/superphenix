package disk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"

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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListDisks
//
//	@Summary		Retrieve all disks
//	@Description	Retrieve all disks across AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string				true	"Organization ID"
//	@Param			projectId	path	string				true	"Project ID"
//	@Success		200			{array}	DiskFullResponse	"Disks"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{projectId}/disk [get]
//	@Security		Bearer[OrganizationRead, ProjectDiskRead]
func (h *Service) ListDisks(w http.ResponseWriter, r *http.Request) {
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

	resourcesDb, err := product.FindAllByProjectIdAndResourceType(project.ID.String(), model.ProductTypeDisk)
	if err != nil {
		log.Err(err).Str("projectId", project.ID.String()).Str("resourceType", model.ProductTypeDisk).Msg(consts.SpxFindAllResourcesError)
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

// ListAZDisks
//
//	@Summary		Retrieve all AZ disks
//	@Description	Retrieve all disks for a specific AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string				true	"Organization ID"
//	@Param			az			path	string				true	"AZ Code"
//	@Param			projectId	path	string				true	"Project ID"
//	@Success		200			{array}	DiskFullResponse	"Disks"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/disk [get]
//	@Security		Bearer[OrganizationRead, ProjectDiskRead]
func (h *Service) ListAZDisks(w http.ResponseWriter, r *http.Request) {
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

	resources, err := product.FindAllByProjectIdAndResourceTypeAndCodeAZ(projectDb.ID.String(), model.ProductTypeDisk, azDb.Code)
	if err != nil {
		log.Err(err).
			Str("projectId", projectDb.ID.String()).
			Str("resourceType", model.ProductTypeDisk).
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

// GetDisk
//
//	@Summary		Get disk
//	@Description	Get disk by Effective ID
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path		string				true	"Organization ID"
//	@Param			az			path		string				true	"AZ Code"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			effectiveId	path		string				true	"Disk EID"
//	@Success		200			{object}	DiskFullResponse	"Disk"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/disk/{effectiveId} [get]
//	@Security		Bearer[OrganizationRead, ProjectDiskRead]
func (h *Service) GetDisk(w http.ResponseWriter, r *http.Request) {
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
	var result DiskFullResponse

	// If we didn't find spx-ctrl, but we got db info
	if (resp.StatusCode != http.StatusOK) && dbErr == nil {
		result = DiskFullResponse{
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
			log.Info().Ctx(r.Context()).Err(dbErr).Str("resourceEId", resourceEId).Msg("Resource not found in DB")

			result = DiskFullResponse{
				ProductResponse: ProductResponse{
					ID:          mapResult["id"].(string),
					EId:         mapResult["eid"].(string),
					ProductName: mapResult["productName"].(string),
					CodeAZ:      azDb.Code,
					Gitops:      mapResult["gitops"].(string),
				},
				Disk:        mapResult["disk"],
				PVC:         mapResult["pvc"],
				MountStatus: mapResult["mountStatus"],
			}
		} else {
			//	We got both db and spx-ctrl info
			if dbProduct.ID.String() == mapResult["id"] {
				result = DiskFullResponse{
					ProductResponse: ProductResponse{
						ID:            dbProduct.ID.String(),
						EId:           mapResult["eid"].(string),
						ProductName:   dbProduct.ProductName,
						CodeAZ:        azDb.Code,
						ProductTypeId: dbProduct.ProductTypeId,
						Gitops:        mapResult["gitops"].(string),
					},
					Disk:        mapResult["disk"],
					PVC:         mapResult["pvc"],
					MountStatus: mapResult["mountStatus"],
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

// CreateDisk
//
//	@Summary		Create disk
//	@Description	Create a new disk and start it
//	@Tags			v1, Superphenix Controller
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path		string			true	"Organization ID"
//	@Param			az			path		string			true	"AZ Code"
//	@Param			projectId	path		string			true	"Project ID"
//	@Param			Body		body		CreateDiskBody	true	"Disk info"
//	@Success		200			{object}	controller.CreateResponse
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/disk [post]
//	@Security		Bearer[OrganizationRead, ProjectDiskWrite]
func (h *Service) CreateDisk(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, org, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	// Create the product in DB
	var body CreateDiskBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	if v, err := strconv.Atoi(body.General.Storage); err != nil || v > 5000 {
		reason := fmt.Errorf("storage size is bigger than 5000")
		if err != nil {
			reason = fmt.Errorf("failed to convert storage size")
		}

		log.Err(reason).Msg("Create Disk Body not valid")
		httpError.Http(w, r, http.StatusBadRequest).Str("reason", reason.Error()).Msg(http.StatusText(http.StatusBadRequest))
		return
	}

	disk, m, err := controller.CreateIntoDb(r.Context(), body.General.ProductName, model.ProductTypeDisk, azDb.Code, org.ID, projectEntity.ID)
	if err != nil {
		log.Err(err).Msg("Failed to save product into database")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	// Send request to superphenix-controller
	newBody := CreateDiskSpxControllerBody{
		Metadata: m,
		General: struct {
			Storage string `json:"storage"`
			Source  struct {
				Type     string `json:"type"`
				URL      string `json:"url,omitempty"`
				Clone    string `json:"clone,omitempty"`
				Snapshot string `json:"snapshot,omitempty"`
			} `json:"source"`
			StorageClass string   `json:"storageClass"`
			Labels       []string `json:"labels,omitempty"`
		}{
			Storage:      body.General.Storage,
			Source:       body.General.Source,
			StorageClass: body.General.StorageClass,
			Labels:       body.General.Labels,
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
		controller.WriteCreateResponse(w, disk.EffectiveID)
	} else {
		ctrlutils.CleanDb(r.Context(), disk.ID)
		ctrlutils.HandleControllerError(w, r, resp, consts.SpxResourceCreationFailureCode, consts.SpxResourceCreationFailure)
		return
	}
}

// UpdateDisk
//
//	@Summary		Update disk
//	@Description	Update a disk
//	@Tags			v1, Superphenix Controller
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path	string			true	"Organization ID"
//	@Param			az			path	string			true	"AZ Code"
//	@Param			projectId	path	string			true	"Project ID"
//	@Param			effectiveId	path	string			true	"Instance EID"
//	@Param			force		query	boolean			false	"Temporary: force update of a gitops-managed disk"
//	@Param			Body		body	UpdateDiskBody	true	"Disk info"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/disk/{effectiveId} [post]
//	@Security		Bearer[OrganizationRead, ProjectDiskWrite]
func (h *Service) UpdateDisk(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, _, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	var body UpdateDiskBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	productEid := chi.URLParam(r, "effectiveId")
	if _, err := controller.UpdateIntoDb(r.Context(), productEid, body.General.ProductName, model.ProductTypeDisk, azDb.Code, projectEntity.ID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
			return
		}
		log.Error().Err(err).Msg("Failed to save product in database")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}

	// Send request to superphenix-controller
	newBody := UpdateDiskSpxControllerBody{
		General: struct {
			Storage string   `json:"storage"`
			Labels  []string `json:"labels,omitempty"`
		}{
			Storage: body.General.Storage,
			Labels:  body.General.Labels,
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
		w.WriteHeader(http.StatusOK)
	} else if resp.StatusCode == http.StatusNotFound {
		httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
		return
	} else {
		ctrlutils.HandleControllerError(w, r, resp, consts.SpxResourceUpdateFailureCode, consts.SpxResourceUpdateFailure)
		return
	}
}

// DeleteDisk
//
//	@Summary		Delete disk
//	@Description	Delete disk by effective ID
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Disk EID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/disk/{effectiveId} [delete]
//	@Security		Bearer[OrganizationRead, ProjectDiskWrite]
func (h *Service) DeleteDisk(w http.ResponseWriter, r *http.Request) {
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
		resourceEId := chi.URLParam(r, "effectiveId")
		rowsAffected, err := product.DeleteByEIdAndAZCodeAndProject(resourceEId, azDb.Code, projectEntity.ID)
		if err != nil {
			log.Err(err).Msg(consts.SpxResourceDeletionFailure)
			httpError.Http(w, r, consts.SpxResourceDeletionFailureCode).Msg(consts.SpxResourceDeletionFailure)
			return
		}
		if resp.StatusCode == http.StatusNotFound && rowsAffected == 0 {
			httpError.Http(w, r, http.StatusNotFound).Str("eid", resourceEId).Msg(consts.SpxResourceNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	} else {
		ctrlutils.HandleControllerError(w, r, resp, consts.SpxResourceDeletionFailureCode, consts.SpxResourceDeletionFailure)
		return
	}
}

// combineListResult regroup results from db and controller
func combineListResult(concatResults map[string][]interface{}, resources []model.Product, mapResourceCheck map[uuid.UUID]bool) []DiskFullResponse {
	combineResults := make([]DiskFullResponse, 0)
	for azCode, results := range concatResults {
		for _, result := range results {
			mapResult := result.(map[string]interface{})
			found := false
			for _, p := range resources {
				if p.ID.String() == mapResult["id"] {
					combineResults = append(combineResults, DiskFullResponse{
						ProductResponse: ProductResponse{
							ID:            p.ID.String(),
							EId:           mapResult["eid"].(string),
							ProductName:   p.ProductName,
							CodeAZ:        azCode, // we use az code to handle instance under PRA
							ProductTypeId: p.ProductTypeId,
							Gitops:        mapResult["gitops"].(string),
						},
						Disk:        mapResult["disk"],
						PVC:         mapResult["pvc"],
						MountStatus: mapResult["mountStatus"],
					})
					mapResourceCheck[p.ID] = true
					found = true
					break
				}
			}

			// If only gitops
			if !found {
				combineResults = append(combineResults, DiskFullResponse{
					ProductResponse: ProductResponse{
						ID:          mapResult["id"].(string),
						EId:         mapResult["eid"].(string),
						ProductName: mapResult["productName"].(string),
						CodeAZ:      azCode,
						Gitops:      mapResult["gitops"].(string),
					},
					Disk:        mapResult["disk"],
					PVC:         mapResult["pvc"],
					MountStatus: mapResult["mountStatus"],
				})
			}
		}

	}

	// Check for not found resources
	for _, p := range resources {
		if mapResourceCheck[p.ID] == false {
			combineResults = append(combineResults, DiskFullResponse{
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

	slices.SortFunc(combineResults, func(a, b DiskFullResponse) int {
		return controller.CompareProductResult(a.ProductResponse, b.ProductResponse)
	})

	return combineResults
}

// ProductResponse is the shared response base defined by the controller kit.
type ProductResponse = controller.ProductResponse

// CreateDiskBody and CreateDiskSpxControllerBody stay in the controller kit
// because instance creation reuses them; alias them here for local use.
type CreateDiskBody = controller.CreateDiskBody
type CreateDiskSpxControllerBody = controller.CreateDiskSpxControllerBody

type UpdateDiskBody struct {
	General struct {
		ProductName string   `json:"productName" validate:"max=63"`
		Storage     string   `json:"storage"` // Storage Size
		Labels      []string `json:"labels,omitempty"`
	} `json:"general"`
}

// UpdateDiskSpxControllerBody is the body send to superphenix-controller to update a Disk
type UpdateDiskSpxControllerBody struct {
	General struct {
		Storage string   `json:"storage"` // Storage Size
		Labels  []string `json:"labels,omitempty"`
	} `json:"general"`
}

type DiskFullResponse struct {
	ProductResponse `json:",inline"`
	Disk            interface{} `json:"disk"`
	PVC             interface{} `json:"pvc"`
	MountStatus     interface{} `json:"mountStatus"`
}

// Unmount
//
//	@Summary		Unmount Disk
//	@Description	Unmount a specific disk
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Disk EID"
//	@Success		200
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/disk/{effectiveId}/unmount [get]
//	@Security		Bearer[OrganizationRead, ProjectInstanceRead, ProjectDiskRead, ProjectInstanceWrite]
func (h *Service) Unmount(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}
