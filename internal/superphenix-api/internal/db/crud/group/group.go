package group

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

	v1 "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1"

	"github.com/google/uuid"
)

func Save(group model.Group) (model.Group, error) {
	result := db.Client.Save(&group)
	return group, result.Error
}

func DeleteById(groupId uuid.UUID) error {
	result := db.Client.Delete(&model.Group{}, groupId)
	return result.Error
}

func DeleteAllByOrgaId(orgaId string) error {
	orgaUuid, _ := uuid.Parse(orgaId)
	result := db.Client.Where("orga_id = ?", orgaUuid).Delete(&model.Group{})
	return result.Error
}

func FindAllByOrgaId(orgaId string) ([]httpModel.APIGroup, error) {
	orgaUuid, _ := uuid.Parse(orgaId)
	return crud.FindAll[model.Group, httpModel.APIGroup](model.Group{
		OrgaId: orgaUuid,
	})
}

func FindOwnerGroup(orgId string) (model.Group, error) {
	orgUuid, _ := uuid.Parse(orgId)

	return crud.Find[model.Group, model.Group](model.Group{
		OrgaId: orgUuid,
		Name:   v1.DefaultGroupOwnerName,
	})
}

func FindAllByOrgaIdExceptOwner(orgaId string) ([]httpModel.APIGroup, error) {
	orgaUuid, _ := uuid.Parse(orgaId)

	modelGroup := model.Group{
		OrgaId: orgaUuid,
	}

	query := db.Client.Model(&modelGroup).Where(&modelGroup).Not(&model.Group{Name: v1.DefaultGroupOwnerName})

	var list []model.Group
	result := query.Find(&list)

	cast := make([]httpModel.APIGroup, 0)
	if result.RowsAffected == 0 {
		return cast, result.Error
	}

	temporaryVariable, err := json.Marshal(list)
	err = json.Unmarshal(temporaryVariable, &cast)
	return cast, err
}

func FindAllByOrgaIdAndAllProjects(orgaId string) ([]model.Group, error) {
	orgaUuid, _ := uuid.Parse(orgaId)
	return crud.FindAll[model.Group, model.Group](model.Group{
		OrgaId:      orgaUuid,
		AllProjects: true,
	})
}

func FindById(groupId string) (model.Group, error) {
	groupUuid, _ := uuid.Parse(groupId)
	return crud.Find[model.Group, model.Group](model.Group{
		Model: model.Model{
			ID: groupUuid,
		},
	})
}

func FindByIdAndOrgaId(groupId, orgaId string) (httpModel.APIGroup, error) {
	groupUuid, _ := uuid.Parse(groupId)
	orgaUuid, _ := uuid.Parse(orgaId)
	return crud.Find[model.Group, httpModel.APIGroup](model.Group{
		Model: model.Model{
			ID: groupUuid,
		},
		OrgaId: orgaUuid,
	})
}

func FindAllByIdsAndOrgaId(groupIds []string, orgaId string) ([]httpModel.APIGroup, error) {
	orgaUuid, _ := uuid.Parse(orgaId)
	var list []httpModel.APIGroup
	res := db.Client.Model(&model.Group{}).Where(&model.Group{OrgaId: orgaUuid}).Find(&list, groupIds)
	return list, res.Error
}

func CountUserAssignedToGroup(groupId, orgaId string) (int64, error) {
	groupUuid, _ := uuid.Parse(groupId)
	orgaUuid, _ := uuid.Parse(orgaId)
	sample := model.UserOrganization{
		GroupId:        groupUuid,
		OrganizationId: orgaUuid,
	}

	var count int64
	res := db.Client.Model(&sample).Where(&sample).Count(&count)
	return count, res.Error
}

// IsQuotaCreationReached compare quota with number of project inside the organization
// Return true if the quota is reached
func IsQuotaCreationReached(ctx context.Context, orgaId uuid.UUID) (bool, error) {
	log := logger.GetLogger(ctx)
	limitQuota, err := quota.GetQuotaValue(quota.OrgaLimitIAMGroup, orgaId, quota.MapQuotaEntityType[quota.OrgaLimitIAMGroup])
	if err != nil {
		log.Err(err).Str("orgaId", orgaId.String()).Msg("failed to get quota")
		return false, err
	}

	var count int64
	res := db.Client.Model(&model.Group{}).Where(&model.Group{OrgaId: orgaId}).Count(&count)
	if res.Error != nil {
		log.Err(err).Str("orgaId", orgaId.String()).Msg("failed to count projects")
		return false, err
	}

	limit, err := strconv.ParseInt(limitQuota.Value, 10, 64)
	if err != nil {
		log.Err(err).Any("limitQuota", limitQuota).Msg("failed to parse quota value")
		return false, err
	}

	return count >= limit, nil
}
