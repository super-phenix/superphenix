package project

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/group"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/product"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/project"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/quotasync"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/gc"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/audit"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	pw "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CreateOrUpdateProjectBody struct {
	Id   string `json:"id"`
	Name string `json:"name" validate:"max=63"`
}

// ErrProjectNotInOrga is returned when a project does not belong to the
// organization it is being addressed under. Callers map it to a 404 so that a
// cross-organization reference is indistinguishable from a missing project.
var ErrProjectNotInOrga = errors.New("project not found in organization")

// CreateOrUpdateProject
//
//	@Summary		Create or Update Project
//	@Description	Create or Update Project
//	@Tags			v1, project
//	@Accept			json
//	@Produce		json
//	@Param			orgaId	path		string						true	"Organization ID"
//	@Param			Body	body		CreateOrUpdateProjectBody	true	"Project Info"
//	@Success		200		{object}	model.APIProject			"Project"
//	@Failure		400
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/project [post]
//	@Security		Bearer[OrganizationRead, OrganizationProjectManagement]
func (h *Service) CreateOrUpdateProject(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")
	orgaUuid, _ := uuid.Parse(orgaId)

	userUuid, err := utils.GetUserUuid(r)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user uuid")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var body CreateOrUpdateProjectBody
	if err := decoder.HandleHTTPJSON(w, r, &body, h.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	var p model.APIProject
	audit.SetResource(r.Context(), body.Id)
	if projectUuid, err := uuid.Parse(body.Id); err == nil {
		audit.SetProject(r.Context(), projectUuid)
	}
	if body.Id != "" {
		p, err = project.UpdateProject(r.Context(), orgaUuid, body.Id, body.Name)
		if err != nil {
			log.Error().Err(err).Msg("Failed to update project")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else {
		reached, err := project.IsQuotaCreationReached(r.Context(), orgaUuid)
		if err != nil {
			log.Error().Err(err).Msg("Failed to check quota")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if reached {
			log.Error().Msg("Quota to create project reached")
			http.Error(w, "Quota to create project reached", http.StatusBadRequest)
			return
		}

		p, err = InitializeProject(r.Context(), userUuid, orgaUuid, body.Name)
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize project")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		audit.SetResource(r.Context(), p.ID.String())
		audit.SetProject(r.Context(), p.ID)
	}

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(p)
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling response")
	}
	_, _ = w.Write(marshal)

}

// DeleteProject
//
//	@Summary		Delete Project
//	@Description	Delete Project
//	@Tags			v1, project
//	@Produce		json
//	@Param			orgaId		path	string	true	"Organization ID"
//	@Param			projectId	query	string	true	"Project ID"
//	@Success		200
//	@Failure		400
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/project [delete]
//	@Security		Bearer[OrganizationRead, OrganizationProjectManagement]
func (h *Service) DeleteProject(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")
	projectId, ok := ch.GetQuery(r, "projectId")
	if !ok {
		log.Error().Msg("No project id found")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if projectUuid, err := uuid.Parse(projectId); err == nil {
		audit.SetProject(r.Context(), projectUuid)
	}

	if err := RemoveProject(r.Context(), orgaId, projectId); err != nil {
		if errors.Is(err, ErrProjectNotInOrga) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)

	}

}

// RemoveProject delete the project and all associated resources
func RemoveProject(ctx context.Context, orgaId, projectId string) error {
	log := logger.GetLogger(ctx)

	orgaUuid, err := uuid.Parse(orgaId)
	if err != nil {
		log.Error().Err(err).Str("orgaId", orgaId).Msg("Invalid organization id")
		return err
	}

	projectUuid, err := uuid.Parse(projectId)
	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msg("Invalid project id")
		return err
	}

	// Authorization guard: ensure the project belongs to this organization
	// before performing any destructive action. Without this check, a caller
	// authorized on their own organization could delete any project by id
	// (cross-organization IDOR).
	if _, err := project.FindByIdAndOrgaId(projectUuid, orgaUuid); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn().Str("orgaId", orgaId).Str("projectId", projectId).Msg("Project not found in organization")
			return ErrProjectNotInOrga
		}
		log.Error().Err(err).Str("orgaId", orgaId).Str("projectId", projectId).Msg("Failed to look up project")
		return err
	}

	// Mark all Resources for deletion
	if err := gc.MarkResources(ctx, orgaId, projectId); err != nil {
		log.Error().Err(err).Msg("Failed to mark project's products for deletion")
		return err
	}

	// Delete resources in database
	if err := product.DeleteByProject(projectUuid, orgaUuid); err != nil {
		log.Error().Err(err).Msg("Failed to delete products in project")
		return err
	}

	// Delete project in database
	if err := project.DeleteById(projectUuid, orgaUuid); err != nil {
		log.Error().Err(err).Msg("Failed to delete project")
		return err
	}

	// Remove Permify permission
	if err := pw.DeleteRelationProject(ctx, orgaId, projectId); err != nil {
		log.Error().Err(err).Msg("Failed to delete project from Permify")
		return err
	}

	return nil
}

// InitializeProject create a new project into the database, and init the associated permissions
//
// It is not canceled with ctx: a client disconnecting halfway must not leave a project created
// in the database without its permissions or its quota.
func InitializeProject(ctx context.Context, userId, orgaId uuid.UUID, name string) (model.APIProject, error) {
	ctx = context.WithoutCancel(ctx)
	log := logger.GetLogger(ctx)
	projectEntity, err := project.CreateProject(ctx, orgaId, name)
	if err != nil {
		log.Err(err).Str("userId", userId.String()).Msg("Failed to initialize default project in DB")
		return model.APIProject{}, err

	}

	groups, err := group.FindAllByOrgaIdAndAllProjects(orgaId.String())
	if err != nil {
		log.Err(err).Str("userId", userId.String()).Any("project", projectEntity).Msg("Failed to initialize project permission")
		return model.APIProject{}, err
	}

	var mapGroups = make(pw.MapGroupPermissionSets)
	for _, g := range groups {
		mapGroups[g.ID.String()] = g.PermissionSets
	}
	err = pw.CreateRelationProject(ctx, orgaId.String(), projectEntity.ID.String(), mapGroups)
	if err != nil {
		log.Err(err).Str("userId", userId.String()).Any("project", projectEntity).Msg("Failed to initialize project permission")
		return model.APIProject{}, err
	}

	// A project created in an organization with a quota gets 0 for every limited
	// resource, until a project quota is given to it. When the quota cannot be applied,
	// the creation soft-fails: the project is kept but deactivated.
	quotaErr := quotasync.Initialize(ctx, orgaId, projectEntity.ID)
	if quotaErr != nil {
		log.Err(quotaErr).Str("userId", userId.String()).Any("project", projectEntity).Msg("Failed to apply project quota")
	}

	if quotaErr == nil && config.Global.UserSettings.EnableProjectDefaultResources {
		if err := controller.InitializeProjectDefaultResources(ctx, orgaId, projectEntity.ID); err != nil {
			log.Err(err).Str("userId", userId.String()).Any("project", projectEntity).Msg("Failed to initialize default resources in project")
		}
	}

	// Report the deactivation, if any, to the caller.
	if quotaErr != nil {
		if projectEntity.Deactivations, err = project.ListDeactivations(ctx, orgaId, projectEntity.ID); err != nil {
			log.Err(err).Str("userId", userId.String()).Any("project", projectEntity).Msg("Failed to read project deactivations")
		}
	}

	var cast model.APIProject
	temporaryVariable, err := json.Marshal(projectEntity)
	err = json.Unmarshal(temporaryVariable, &cast)
	return cast, err
}
