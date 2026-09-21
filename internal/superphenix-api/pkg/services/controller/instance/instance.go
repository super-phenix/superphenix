package instance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
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

// ListInstances
//
//	@Summary		Retrieve all instances
//	@Description	Retrieve all instances across AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string					true	"Organization ID"
//	@Param			projectId	path	string					true	"Project ID"
//	@Success		200			{array}	InstanceFullResponse	"Instances"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{projectId}/instance [get]
//	@Security		Bearer[OrganizationRead, ProjectInstanceRead]
func (h *Service) ListInstances(w http.ResponseWriter, r *http.Request) {
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

	resourcesDb, err := product.FindAllByProjectIdAndResourceType(project.ID.String(), model.ProductTypeInstance)
	if err != nil {
		log.Err(err).Str("projectId", project.ID.String()).Str("resourceType", model.ProductTypeInstance).Msg(consts.SpxFindAllResourcesError)
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

// ListAZInstances
//
//	@Summary		Retrieve all AZ instances
//	@Description	Retrieve all instances for a specific AZ
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string					true	"Organization ID"
//	@Param			az			path	string					true	"AZ Code"
//	@Param			projectId	path	string					true	"Project ID"
//	@Success		200			{array}	InstanceFullResponse	"Instances"
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance [get]
//	@Security		Bearer[OrganizationRead, ProjectInstanceRead]
func (h *Service) ListAZInstances(w http.ResponseWriter, r *http.Request) {
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

	resources, err := product.FindAllByProjectIdAndResourceTypeAndCodeAZ(projectDb.ID.String(), model.ProductTypeInstance, azDb.Code)
	if err != nil {
		log.Err(err).
			Str("projectId", projectDb.ID.String()).
			Str("resourceType", model.ProductTypeInstance).
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

// GetInstance
//
//	@Summary		Get instance
//	@Description	Get instance by Effective ID
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path		string					true	"Organization ID"
//	@Param			az			path		string					true	"AZ Code"
//	@Param			projectId	path		string					true	"Project ID"
//	@Param			effectiveId	path		string					true	"Instance EID"
//	@Success		200			{object}	InstanceFullResponse	"Instance"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId} [get]
//	@Security		Bearer[OrganizationRead, ProjectInstanceRead]
func (h *Service) GetInstance(w http.ResponseWriter, r *http.Request) {
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
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			// If the request is not OK or NotFound then log it
			log.Error().Str("path", r.URL.Path).Str("status", resp.Status).Int("statusCode", resp.StatusCode).Str("az", azDb.Code).Msg("Request on superphenix-controller failed")
		}
	}

	resourceEId := chi.URLParam(r, "effectiveId")
	dbProduct, dbErr := product.FindByEId(resourceEId)

	productResponse, azResult, outcome := controller.ResolveProductResponse(r.Context(), azDb.Code, resp, dbProduct, dbErr)

	// If we can't find either spx-ctrl or db info
	if outcome == controller.MergeNotFound {
		log.Error().Str("effectiveId", resourceEId).Msg(consts.SpxResourceNotFound)
		httpError.Http(w, r, http.StatusNotFound).Str("eid", resourceEId).Msg(consts.SpxResourceNotFound)
		return
	}

	result := InstanceFullResponse{ProductResponse: productResponse}
	if azResult != nil {
		result.VM = azResult["vm"]
		result.VMI = azResult["vmi"]
		result.CloudInit = azResult["cloudInit"]
		result.ContainerDisks = resolveMountedContainerDisks(azResult)
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

// CreateInstance
//
//	@Summary		Create instance
//	@Description	Create a new instance and start it
//	@Tags			v1, Superphenix Controller
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path		string				true	"Organization ID"
//	@Param			az			path		string				true	"AZ Code"
//	@Param			projectId	path		string				true	"Project ID"
//	@Param			Body		body		CreateInstanceBody	true	"Instance info"
//	@Success		200			{object}	controller.CreateResponse
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance [post]
//	@Security		Bearer[OrganizationRead, ProjectInstanceWrite]
func (h *Service) CreateInstance(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, org, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	// Create the product in DB
	var body CreateInstanceBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	// Check network values
	if len(body.Network) <= 0 {
		log.Error().Any("body", body).Msg("Network is required")
		httpError.Http(w, r, http.StatusBadRequest).Any("reason", "Network is required").Msg(http.StatusText(http.StatusBadRequest))
		return
	}

	instance, m, err := controller.CreateIntoDb(r.Context(), body.General.ProductName, model.ProductTypeInstance, azDb.Code, org.ID, projectEntity.ID)
	if err != nil {
		log.Err(err).Msg("Failed to save product into database")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	// Initialize new disks
	disksCreated, disksForCtrl, err := initializeNewDisks(r.Context(), org.ID, projectEntity.ID, azDb.Code, body.Disks)
	// If something went wrong, clean all disks and the instance from db
	if err != nil {
		for _, disk := range disksCreated {
			err := product.DeleteById(disk.ID)
			if err != nil {
				log.Error().Err(err).Any("disk", disk).Msg("Failed to delete product")
			}
		}
		err := product.DeleteById(instance.ID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to delete product")
		}
		log.Err(err).Msg("Failed to initialize associated disks into database")
		httpError.Http(w, r, consts.SpxResourceCreationFailureCode).Msg(consts.SpxResourceCreationFailure)
		return
	}

	// Container disk IDs are forwarded as-is; the controller validates them
	// (an unknown id returns a 400, handled below where the created rows are cleaned up).
	newBody := CreateInstanceSpxControllerBody{
		Metadata: m,
		General: struct {
			RunStrategy string   `json:"runStrategy"`
			VMType      string   `json:"vmType"`
			Labels      []string `json:"labels"`
		}{
			RunStrategy: body.General.RunStrategy,
			VMType:      body.General.VMType,
			Labels:      body.General.Labels,
		},
		Compute:        body.Compute,
		Network:        body.Network,
		Disks:          disksForCtrl,
		CloudInit:      body.CloudInit,
		SSHKeys:        body.SSHKeys,
		ContainerDisks: body.ContainerDisks,
		Advanced:       body.Advanced,
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
		controller.WriteCreateResponse(w, instance.EffectiveID)
	} else {
		for _, disk := range disksCreated {
			ctrlutils.CleanDb(r.Context(), disk.ID)
		}
		ctrlutils.CleanDb(r.Context(), instance.ID)
		ctrlutils.HandleControllerError(w, r, resp, consts.SpxResourceCreationFailureCode, consts.SpxResourceCreationFailure)
		return
	}
}

// UpdateInstance
//
//	@Summary		Update instance
//	@Description	Update an instance
//	@Tags			v1, Superphenix Controller
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path	string				true	"Organization ID"
//	@Param			az			path	string				true	"AZ Code"
//	@Param			projectId	path	string				true	"Project ID"
//	@Param			effectiveId	path	string				true	"Instance EID"
//	@Param			Body		body	UpdateInstanceBody	true	"Instance info"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId} [post]
//	@Security		Bearer[OrganizationRead, ProjectInstanceWrite]
func (h *Service) UpdateInstance(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	// Fetch and check all required information
	azDb, org, projectEntity, code, errMsg := ctrlutils.CheckPathParams(r)
	if code != 0 {
		httpError.Http(w, r, code).Msg(errMsg)
		return
	}

	// Create the product in DB
	var body UpdateInstanceBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	// Check network values
	if len(body.Network) <= 0 {
		httpError.Http(w, r, http.StatusBadRequest).Any("body", body).Msg("Network is required")
		return
	}
	productEid := chi.URLParam(r, "effectiveId")
	if _, err := controller.UpdateIntoDb(r.Context(), productEid, body.General.ProductName, model.ProductTypeInstance, azDb.Code, projectEntity.ID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
			return
		}
		log.Error().Err(err).Msg("Failed to save product in database")
		httpError.Http(w, r, consts.SpxResourceUpdateFailureCode).Msg(consts.SpxResourceUpdateFailure)
		return
	}
	// Initialize new disks
	disksCreated, disksForCtrl, err := initializeNewDisks(r.Context(), org.ID, projectEntity.ID, azDb.Code, body.Disks)
	// If something went wrong, clean all disks created from db
	if err != nil {
		for _, disk := range disksCreated {
			err := product.DeleteById(disk.ID)
			if err != nil {
				log.Error().Err(err).Any("disk", disk).Msg("Failed to delete product")
			}
		}
	}

	// Container disk IDs are forwarded as-is for the controller to validate
	// (nil preserves the current mounts).
	newBody := toUpdateAzControllerBody(body, disksForCtrl)

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
	} else {
		for _, disk := range disksCreated {
			if err := product.DeleteById(disk.ID); err != nil {
				log.Error().Err(err).Any("disk", disk).Msg("Failed to delete product")
			}
		}

		if resp.StatusCode == http.StatusNotFound {
			httpError.Http(w, r, http.StatusNotFound).Str("eid", productEid).Msg(consts.SpxResourceNotFound)
			return
		}
		ctrlutils.HandleControllerError(w, r, resp, consts.SpxResourceUpdateFailureCode, consts.SpxResourceUpdateFailure)
		return
	}
}

// DeleteInstance
//
//	@Summary		Delete instance
//	@Description	Delete instance by effective ID
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Instance EID"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId} [delete]
//	@Security		Bearer[OrganizationRead, ProjectInstanceWrite]
func (h *Service) DeleteInstance(w http.ResponseWriter, r *http.Request) {
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

// toUpdateAzControllerBody converts the public UpdateInstanceBody into the az-controller payload.
func toUpdateAzControllerBody(body UpdateInstanceBody, disksForCtrl []InstanceDiskSpxControllerBody) UpdateInstanceSpxControllerBody {
	return UpdateInstanceSpxControllerBody{
		General: struct {
			RunStrategy string   `json:"runStrategy"`
			VMType      string   `json:"vmType"`
			Labels      []string `json:"labels"`
		}{
			RunStrategy: body.General.RunStrategy,
			VMType:      body.General.VMType,
			Labels:      body.General.Labels,
		},
		Compute:        body.Compute,
		Network:        body.Network,
		Disks:          disksForCtrl,
		CloudInit:      body.CloudInit,
		SSHKeys:        body.SSHKeys,
		ContainerDisks: body.ContainerDisks,
		Advanced:       body.Advanced,
	}
}

// resolveMountedContainerDisks returns the name of every volume whose source
// is a ContainerDisk. The volume name is the catalog ID.
func resolveMountedContainerDisks(mapResult map[string]interface{}) []string {
	volumes := extractRawVolumes(mapResult)
	out := make([]string, 0, len(volumes))
	for _, vol := range volumes {
		volMap, ok := vol.(map[string]interface{})
		if !ok {
			continue
		}
		if _, isCD := volMap["containerDisk"].(map[string]interface{}); !isCD {
			continue
		}
		if name, _ := volMap["name"].(string); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func extractRawVolumes(mapResult map[string]interface{}) []interface{} {
	vm, ok := mapResult["vm"].(map[string]interface{})
	if !ok {
		return nil
	}
	spec, ok := vm["spec"].(map[string]interface{})
	if !ok {
		return nil
	}
	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return nil
	}
	templateSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return nil
	}
	volumes, _ := templateSpec["volumes"].([]interface{})
	return volumes
}

func initializeNewDisks(ctx context.Context, orgID, projectID uuid.UUID, azCode string, disks []InstanceDiskBody) (diskCreated []model.Product, diskForCtrl []InstanceDiskSpxControllerBody, err error) {
	log := logger.GetLogger(ctx)
	// Initialize new disks
	disksCreated := make([]model.Product, 0)
	disksForCtrl := make([]InstanceDiskSpxControllerBody, 0)

	for _, diskToCreate := range disks {
		if !reflect.DeepEqual(diskToCreate.Disk, CreateDiskBody{}) {
			//Check quota
			reached, err := product.IsQuotaCreationReached(ctx, projectID)
			if err != nil {
				log.Error().Err(err).Msg("Failed to check quota")
				return disksCreated, disksForCtrl, fmt.Errorf("failed to check quota")
			}
			if reached {
				log.Error().Msg("Quota to create product reached")
				return disksCreated, disksForCtrl, fmt.Errorf("quota to create product reached")
			}

			diskCreated, err := product.Save(model.Product{
				ProductName:   diskToCreate.Disk.General.ProductName,
				CodeAZ:        azCode,
				ProjectId:     projectID,
				ProductTypeId: model.ProductTypeDisk,
			})
			if err != nil {
				log.Error().Err(err).Any("disk", diskToCreate).Msg("Failed to save disk")
				return disksCreated, disksForCtrl, err
			}
			disksCreated = append(disksCreated, diskCreated)

			diskId := diskCreated.ID
			m := spxId.Metadata{}
			if err = m.GenerateMetadata(projectID.String(), orgID.String(), diskId.String()); err != nil {
				return disksCreated, disksForCtrl, err
			}

			diskCreated.EffectiveID = m.GetResourceEffectiveID()
			diskCreated, err = product.Save(diskCreated)
			if err != nil {
				return disksCreated, disksForCtrl, err
			}

			disksForCtrl = append(disksForCtrl, InstanceDiskSpxControllerBody{
				Order: diskToCreate.Order,
				Cdrom: diskToCreate.Cdrom,
				Bus:   diskToCreate.Bus,
				Disk: CreateDiskSpxControllerBody{
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
						Storage:      diskToCreate.Disk.General.Storage,
						Source:       diskToCreate.Disk.General.Source,
						StorageClass: diskToCreate.Disk.General.StorageClass,
					},
				},
			})
		} else {
			disksForCtrl = append(disksForCtrl, InstanceDiskSpxControllerBody{
				Order: diskToCreate.Order,
				Cdrom: diskToCreate.Cdrom,
				Bus:   diskToCreate.Bus,
				Eid:   diskToCreate.Eid,
			})
		}
	}
	return disksCreated, disksForCtrl, nil
}

// combineListResult regroup results from db and controller
func combineListResult(concatResults map[string][]interface{}, resources []model.Product, mapResourceCheck map[uuid.UUID]bool) []InstanceFullResponse {
	combineResults := make([]InstanceFullResponse, 0)
	for azCode, results := range concatResults {
		for _, result := range results {
			mapResult := result.(map[string]interface{})
			found := false
			for _, p := range resources {
				if p.ID.String() == mapResult["id"] {
					combineResults = append(combineResults, InstanceFullResponse{
						ProductResponse: ProductResponse{
							ID:            p.ID.String(),
							EId:           mapResult["eid"].(string),
							ProductName:   p.ProductName,
							CodeAZ:        azCode, // we use az code to handle instance under PRA
							ProductTypeId: p.ProductTypeId,
							Gitops:        mapResult["gitops"].(string),
						},
						VM:  mapResult["vm"],
						VMI: mapResult["vmi"],
					})
					mapResourceCheck[p.ID] = true
					found = true
					break
				}
			}

			// If only gitops
			if !found {
				combineResults = append(combineResults, InstanceFullResponse{
					ProductResponse: ProductResponse{
						ID:          mapResult["id"].(string),
						EId:         mapResult["eid"].(string),
						ProductName: mapResult["productName"].(string),
						CodeAZ:      azCode,
						Gitops:      mapResult["gitops"].(string),
					},
					VM:  mapResult["vm"],
					VMI: mapResult["vmi"],
				})
			}
		}

	}

	// Check for not found resources
	for _, p := range resources {
		if mapResourceCheck[p.ID] == false {
			combineResults = append(combineResults, InstanceFullResponse{
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

	slices.SortFunc(combineResults, func(a, b InstanceFullResponse) int {
		return controller.CompareProductResult(a.ProductResponse, b.ProductResponse)
	})

	return combineResults
}

// ProductResponse is the shared response base defined by the controller kit.
type ProductResponse = controller.ProductResponse

// CreateDiskBody and CreateDiskSpxControllerBody stay in the controller kit
// because disk creation reuses them; alias them here for local use.
type CreateDiskBody = controller.CreateDiskBody
type CreateDiskSpxControllerBody = controller.CreateDiskSpxControllerBody

type CreateInstanceBody struct {
	General struct {
		ProductName string   `json:"productName" validate:"max=63"`
		RunStrategy string   `json:"runStrategy"`
		VMType      string   `json:"vmType"`
		Labels      []string `json:"labels"`
	} `json:"general"`
	Compute struct {
		Cpu    int `json:"cpu"`
		Memory int `json:"memory"`
	} `json:"compute"`
	Network   []InstanceNetworkBody `json:"network" validate:"required,dive"`
	Disks     []InstanceDiskBody    `json:"disks"`
	CloudInit InstanceCloudInitBody `json:"cloudInit"`
	SSHKeys   []string              `json:"sshKeys,omitempty"`
	// Nil uses the recommended set; a non-nil slice (even empty) is used as-is.
	ContainerDisks *[]string `json:"containerDisks,omitempty"`
	// Advanced is optional: nil pushes no device/firmware override.
	Advanced *InstanceAdvancedOptionsBody `json:"advanced,omitempty"`
}

// CreateInstanceSpxControllerBody is the body send to superphenix-controller to create an Instance
type CreateInstanceSpxControllerBody struct {
	spxId.Metadata
	General struct {
		RunStrategy string   `json:"runStrategy"`
		VMType      string   `json:"vmType"`
		Labels      []string `json:"labels"`
	} `json:"general"`
	Compute struct {
		Cpu    int `json:"cpu"`
		Memory int `json:"memory"`
	} `json:"compute"`
	Network        []InstanceNetworkBody           `json:"network"`
	Disks          []InstanceDiskSpxControllerBody `json:"disks"`
	CloudInit      InstanceCloudInitBody           `json:"cloudInit"`
	SSHKeys        []string                        `json:"sshKeys,omitempty"`
	ContainerDisks *[]string                       `json:"containerDisks,omitempty"`
	Advanced       *InstanceAdvancedOptionsBody    `json:"advanced,omitempty"`
}

type UpdateInstanceBody struct {
	General struct {
		ProductName string   `json:"productName" validate:"max=63"`
		RunStrategy string   `json:"runStrategy"`
		VMType      string   `json:"vmType"`
		Labels      []string `json:"labels"`
	} `json:"general"`
	Compute struct {
		Cpu    int `json:"cpu"`
		Memory int `json:"memory"`
	} `json:"compute"`
	Network   []InstanceNetworkBody `json:"network" validate:"required,dive"`
	Disks     []InstanceDiskBody    `json:"disks"`
	CloudInit InstanceCloudInitBody `json:"cloudInit"`
	SSHKeys   []string              `json:"sshKeys,omitempty"`
	// Nil preserves the current mounts; a non-nil slice sets them (empty detaches all).
	ContainerDisks *[]string `json:"containerDisks,omitempty"`
	// Advanced is optional: nil leaves existing device/firmware overrides untouched.
	Advanced *InstanceAdvancedOptionsBody `json:"advanced,omitempty"`
}

// UpdateInstanceSpxControllerBody is the body send to superphenix-controller to create an Instance
type UpdateInstanceSpxControllerBody struct {
	General struct {
		RunStrategy string   `json:"runStrategy"`
		VMType      string   `json:"vmType"`
		Labels      []string `json:"labels"`
	} `json:"general"`
	Compute struct {
		Cpu    int `json:"cpu"`
		Memory int `json:"memory"`
	} `json:"compute"`
	Network        []InstanceNetworkBody           `json:"network"`
	Disks          []InstanceDiskSpxControllerBody `json:"disks"`
	CloudInit      InstanceCloudInitBody           `json:"cloudInit"`
	SSHKeys        []string                        `json:"sshKeys,omitempty"`
	ContainerDisks *[]string                       `json:"containerDisks,omitempty"`
	Advanced       *InstanceAdvancedOptionsBody    `json:"advanced,omitempty"`
}

// BlockToggle is the enable baseline every advanced-options block embeds: a
// tri-state toggle (nil=inherit, true=on, false=off). The anonymous embed
// promotes `enabled` into the block's own JSON object.
type BlockToggle struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// TPMOptionsBody overrides the vTPM device (nil=inherit, true=add, false=remove).
type TPMOptionsBody struct {
	BlockToggle
	Persistent *bool `json:"persistent,omitempty"`
}

// EFIOptionsBody overrides the EFI firmware (enabled: nil=inherit, true=EFI, false=BIOS).
type EFIOptionsBody struct {
	BlockToggle
	SecureBoot *bool `json:"secureBoot,omitempty"`
	Persistent *bool `json:"persistent,omitempty"`
}

type AdvancedDevicesBody struct {
	Tpm TPMOptionsBody `json:"tpm"`
}

type AdvancedBootloaderBody struct {
	Efi EFIOptionsBody `json:"efi"`
}

type AdvancedFirmwareBody struct {
	Bootloader AdvancedBootloaderBody `json:"bootloader"`
}

// InstanceAdvancedOptionsBody carries optional device/firmware overrides. Its
// shape mirrors the kubevirt spec tree (spec.template.spec.domain.{devices,firmware}).
// A nil body pushes no override to the controller.
type InstanceAdvancedOptionsBody struct {
	Devices  AdvancedDevicesBody  `json:"devices"`
	Firmware AdvancedFirmwareBody `json:"firmware"`
}

type InstanceNetworkBody struct {
	Order      int    `json:"order"`
	SubnetEId  string `json:"subnetEId"`
	Model      string `json:"model"`
	Enabled    *bool  `json:"enabled,omitempty"`
	IPv4       string `json:"ipv4,omitempty" validate:"omitempty,ipv4"`
	IPv6       string `json:"ipv6,omitempty" validate:"omitempty,ipv6"`
	MACAddress string `json:"macAddress,omitempty" validate:"omitempty,mac"`
}

type InstanceDiskBody struct {
	Order int            `json:"order"`
	Cdrom bool           `json:"cdrom"`
	Bus   string         `json:"bus"`
	Eid   string         `json:"eid,omitempty"`
	Disk  CreateDiskBody `json:"disk,omitempty"`
}

type InstanceCloudInitBody struct {
	Custom bool   `json:"custom"`
	Bus    string `json:"bus"`
	Config string `json:"config,omitempty"`
}

type InstanceDiskSpxControllerBody struct {
	Order int                         `json:"order"`
	Cdrom bool                        `json:"cdrom"`
	Bus   string                      `json:"bus"`
	Eid   string                      `json:"eid,omitempty"`
	Disk  CreateDiskSpxControllerBody `json:"disk,omitempty"`
}

type InstanceFullResponse struct {
	ProductResponse `json:",inline"`
	VM              interface{} `json:"vm"`
	VMI             interface{} `json:"vmi"`
	CloudInit       interface{} `json:"cloudInit"`
	ContainerDisks  []string    `json:"containerDisks"`
}

// AdvancedOptions
//
//	@Summary		Get an instance's advanced options
//	@Description	Resolve the effective advanced device/firmware options of an instance (value + source: vm/preference/default)
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path		string	true	"Organization ID"
//	@Param			az			path		string	true	"AZ Code"
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			effectiveId	path		string	true	"Instance EID"
//	@Success		200			{object}	object	"Advanced options"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId}/advanced-options [get]
//	@Security		Bearer[OrganizationRead, ProjectInstanceRead]
func (h *Service) AdvancedOptions(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}

// Serial
//
//	@Summary		Get Instance serial console
//	@Description	Retrieve serial console information for an instance
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Instance EID"
//	@Success		200
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId}/serial [get]
//	@Security		Bearer[OrganizationRead, ProjectInstanceRead, ProjectInstanceTerminal]
func (h *Service) Serial(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}

// Vnc
//
//	@Summary		Get Instance VNC console
//	@Description	Retrieve VNC console information for an instance
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Instance EID"
//	@Success		200
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId}/vnc [get]
//	@Security		Bearer[OrganizationRead, ProjectInstanceRead, ProjectInstanceTerminal]
func (h *Service) Vnc(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}

// Start
//
//	@Summary		Start Instance
//	@Description	Start a specific instance
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Instance EID"
//	@Success		200
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId}/start [get]
//	@Security		Bearer[OrganizationRead, ProjectInstanceRead, ProjectInstanceControl]
func (h *Service) Start(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}

// Stop
//
//	@Summary		Stop Instance
//	@Description	Stop a specific instance
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Instance EID"
//	@Success		200
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId}/stop [get]
//	@Security		Bearer[OrganizationRead, ProjectInstanceRead, ProjectInstanceControl]
func (h *Service) Stop(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}

// StopForce
//
//	@Summary		Force-stop Instance
//	@Description	Force-stop a specific instance
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Instance EID"
//	@Success		200
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId}/stop-force [get]
//	@Security		Bearer[OrganizationRead, ProjectInstanceRead, ProjectInstanceControl]
func (h *Service) StopForce(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}

// Restart
//
//	@Summary		Restart Instance
//	@Description	Restart a specific instance
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			az			path	string	true	"AZ Code"
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			effectiveId	path	string	true	"Instance EID"
//	@Success		200
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId}/restart [get]
//	@Security		Bearer[OrganizationRead, ProjectInstanceRead, ProjectInstanceControl]
func (h *Service) Restart(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}

// ContainerDiskMount
//
//	@Summary		Mount a batch of container disks on an instance
//	@Description	Mounts a batch of container disks on an instance by their catalog IDs. The IDs are validated against the availability zone's catalog and attached in a single atomic operation.
//	@Tags			v1, Superphenix Controller
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path	string								true	"Organization ID"
//	@Param			az			path	string								true	"AZ Code"
//	@Param			projectId	path	string								true	"Project ID"
//	@Param			effectiveId	path	string								true	"Instance EID"
//	@Param			Body		body	controller.BatchContainerDiskBody	true	"Batch of catalog IDs to mount"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId}/container-disk/mount [post]
//	@Security		Bearer[OrganizationRead, ProjectInstanceRead, ProjectInstanceWrite, ProjectDiskRead, ProjectDiskWrite]
func (h *Service) ContainerDiskMount(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}

// ContainerDiskUnmount
//
//	@Summary		Unmount a batch of container disks from an instance
//	@Description	Unmounts a batch of container disks from an instance by their catalog IDs. The IDs are validated against the availability zone's catalog and detached in a single atomic operation.
//	@Tags			v1, Superphenix Controller
//	@Accept			json
//	@Produce		json
//	@Param			orgaId		path	string								true	"Organization ID"
//	@Param			az			path	string								true	"AZ Code"
//	@Param			projectId	path	string								true	"Project ID"
//	@Param			effectiveId	path	string								true	"Instance EID"
//	@Param			Body		body	controller.BatchContainerDiskBody	true	"Batch of catalog IDs to unmount"
//	@Success		200
//	@Failure		400
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/instance/{effectiveId}/container-disk/unmount [post]
//	@Security		Bearer[OrganizationRead, ProjectInstanceRead, ProjectInstanceWrite, ProjectDiskRead, ProjectDiskWrite]
func (h *Service) ContainerDiskUnmount(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}

// ContainerDisks
//
//	@Summary		List container-disk catalog
//	@Description	Returns the catalog of container-disk types — vendor ISOs (e.g. driver disks) that can be mounted on instances as read-only CDROMs. The available catalog may vary by availability zone.
//	@Tags			v1, Superphenix Controller
//	@Produce		json
//	@Param			orgaId		path	string									true	"Organization ID"
//	@Param			az			path	string									true	"AZ Code"
//	@Param			projectId	path	string									true	"Project ID"
//	@Success		200			{array}	controller.ContainerDiskSpecResponse	"Catalog"
//	@Failure		404
//	@Failure		500
//	@Router			/{orgaId}/api/spx-ctrl/{az}/{projectId}/container-disks [get]
//	@Security		Bearer[OrganizationRead, ProjectDiskRead]
func (h *Service) ContainerDisks(w http.ResponseWriter, r *http.Request) {
	ctrlutils.SimpleRedirect()(w, r)
}
