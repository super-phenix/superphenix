package project

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/group"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/product"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/project"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/gc"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"

	pw "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CreateOrUpdateProjectBody struct {
	Id   string `json:"id"`
	Name string `json:"name" validate:"max=63"`
}

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

	if err := RemoveProject(r.Context(), orgaId, projectId); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)

	}

}

// RemoveProject delete the project and all associated resources
func RemoveProject(ctx context.Context, orgaId, projectId string) error {
	log := logger.GetLogger(ctx)
	projectUuid, _ := uuid.Parse(projectId)

	// Mark all Resources for deletion
	if err := gc.MarkResources(ctx, orgaId, projectId); err != nil {
		log.Error().Err(err).Msg("Failed to mark project's products for deletion")
		return err
	}

	// Delete resources in database
	if err := product.DeleteByProject(projectId); err != nil {
		log.Error().Err(err).Msg("Failed to delete products in project")
		return err
	}

	// Delete project in database
	if err := project.DeleteById(projectUuid); err != nil {
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
func InitializeProject(ctx context.Context, userId, orgaId uuid.UUID, name string) (model.APIProject, error) {
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

	if err := controller.InitializeProjectDefaultResources(ctx, orgaId, projectEntity.ID); err != nil {
		log.Err(err).Str("userId", userId.String()).Any("project", projectEntity).Msg("Failed to initialize default resources in project")
	}

	var cast model.APIProject
	temporaryVariable, err := json.Marshal(projectEntity)
	err = json.Unmarshal(temporaryVariable, &cast)
	return cast, err
}
