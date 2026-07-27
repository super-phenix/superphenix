package project

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	httpModel "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/google/uuid"
)

func FindAllByOrgaId(orgaId string) ([]model.Project, error) {
	orgaUuid, _ := uuid.Parse(orgaId)
	return crud.FindAll[model.Project, model.Project](model.Project{
		OrgaId: orgaUuid,
	})
}

func FindById(projectId string) (httpModel.APIProject, error) {
	projectUuid, _ := uuid.Parse(projectId)
	return crud.Find[model.Project, httpModel.APIProject](model.Project{
		Model: model.Model{
			ID: projectUuid,
		},
	})
}

func DeleteById(projectId uuid.UUID) error {
	result := db.Client.Delete(&model.Project{}, projectId)
	return result.Error
}

// CreateProject create a new project into the database
//
// DO NOT USE OUTSIDE OF PROJECT INITIALIZATION - data.InitializeProject
func CreateProject(ctx context.Context, orgaId uuid.UUID, name string) (model.Project, error) {
	log := logger.GetLogger(ctx)
	newProject := model.Project{
		Name:   name,
		OrgaId: orgaId,
	}

	result := db.Client.Create(&newProject)
	if result.Error != nil {
		log.Error().Ctx(ctx).Err(result.Error).Str("orgaId", orgaId.String()).Str("projectName", name).Any("project", newProject).Msg("Failed to create project in database")
		return model.Project{}, result.Error
	}
	return newProject, nil
}

func UpdateProject(ctx context.Context, orgaId uuid.UUID, projectId, name string) (httpModel.APIProject, error) {
	log := logger.GetLogger(ctx)
	projectUuid, err := uuid.Parse(projectId)
	if err != nil {
		log.Err(err).Ctx(ctx).Str("projectId", projectId).Msg("Failed to parse project id")
		return httpModel.APIProject{}, err
	}

	p, err := crud.Find[model.Project, model.Project](model.Project{
		Model: model.Model{
			ID: projectUuid,
		},
		OrgaId: orgaId,
	})
	if err != nil {
		log.Err(err).Ctx(ctx).Str("orgaId", orgaId.String()).Str("projectId", projectId).Msg("Failed to find project")
		return httpModel.APIProject{}, err
	}

	p.Name = name
	result := db.Client.Save(&p)

	var cast httpModel.APIProject
	temporaryVariable, err := json.Marshal(p)
	err = json.Unmarshal(temporaryVariable, &cast)
	return cast, result.Error
}

// IsQuotaCreationReached compare quota with number of project inside the organization
// Return true if the quota is reached
func IsQuotaCreationReached(ctx context.Context, orgaId uuid.UUID) (bool, error) {
	log := logger.GetLogger(ctx)
	limitQuota, err := quota.GetQuotaValue(quota.OrgaLimitProject, orgaId, quota.MapQuotaEntityType[quota.OrgaLimitProject])
	if err != nil {
		log.Err(err).Any("orgaId", orgaId).Msg("failed to get quota")
		return false, err
	}

	var count int64
	res := db.Client.Model(&model.Project{}).Where(&model.Project{OrgaId: orgaId}).Count(&count)
	if res.Error != nil {
		log.Err(err).Any("orgaId", orgaId).Msg("failed to count projects")
		return false, err
	}

	limit, err := strconv.ParseInt(limitQuota.Value, 10, 64)
	if err != nil {
		log.Err(err).Any("limitQuota", limitQuota).Msg("failed to parse quota value")
		return false, err
	}

	return count >= limit, nil
}
