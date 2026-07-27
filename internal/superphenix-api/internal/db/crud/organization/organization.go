package organization

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/group"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	httpModel "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/google/uuid"
)

func Save(orga model.Organization) (model.Organization, error) {
	result := db.Client.Save(&orga)
	return orga, result.Error
}

func Delete(id string) error {
	orgaUuid, _ := uuid.Parse(id)
	result := db.Client.Select("Projects", "UserRoles").Delete(&model.Organization{
		Model: model.Model{
			ID: orgaUuid,
		},
	})
	if result.Error != nil {
		return result.Error
	}
	return group.DeleteAllByOrgaId(id)
}

func FindById(orgaId string) (model.Organization, error) {
	orgaUuid, _ := uuid.Parse(orgaId)
	return crud.Find[model.Organization, model.Organization](model.Organization{
		Model: model.Model{
			ID: orgaUuid,
		},
	})
}

func FindAPIOrgaById(orgaId string) (httpModel.APIOrganization, error) {
	orgaUuid, _ := uuid.Parse(orgaId)
	return crud.Find[model.Organization, httpModel.APIOrganization](model.Organization{
		Model: model.Model{
			ID: orgaUuid,
		},
	})
}

func FindCompleteOrgById(orgaId string) (httpModel.APIOrganization, error) {
	orgaUuid, _ := uuid.Parse(orgaId)
	org, err := crud.Find[model.Organization, model.Organization](model.Organization{
		Model: model.Model{
			ID: orgaUuid,
		},
	}, "Users", "UserRoles", "Projects")

	if err != nil {
		return httpModel.APIOrganization{}, err
	}

	return castToAPIOrganization(org)
}

func CountOrgaByUser(userUuid uuid.UUID) (int64, error) {
	var count int64
	res := db.Client.Model(&model.Organization{}).Where(&model.Organization{OwnerId: userUuid}).Count(&count)
	return count, res.Error
}

func IsOwner(orgaId, userId string) (bool, error) {
	orgaUuid, _ := uuid.Parse(orgaId)
	userUuid, _ := uuid.Parse(userId)

	org := model.Organization{
		Model:   model.Model{ID: orgaUuid},
		OwnerId: userUuid,
	}

	var count int64
	res := db.Client.Model(&org).Where(&org).Count(&count)
	return count > 0, res.Error
}

func FindWithUsers(orgaId string) (httpModel.APIOrganization, error) {
	orgaUuid, _ := uuid.Parse(orgaId)
	org, err := crud.Find[model.Organization, model.Organization](model.Organization{
		Model: model.Model{
			ID: orgaUuid,
		},
	}, "Users", "UserRoles")

	if err != nil {
		return httpModel.APIOrganization{}, err
	}

	return castToAPIOrganization(org)
}

func FindWithProjects(orgaId string) (httpModel.APIOrganization, error) {
	orgaUuid, _ := uuid.Parse(orgaId)
	return crud.Find[model.Organization, httpModel.APIOrganization](model.Organization{
		Model: model.Model{
			ID: orgaUuid,
		},
	}, "Projects")
}

func InviteUserToOrg(ctx context.Context, orgaId, userToInvite string, groups []uuid.UUID) ([]model.UserOrganization, error) {
	log := logger.GetLogger(ctx)

	userUuid, _ := uuid.Parse(userToInvite)
	orgaUuid, _ := uuid.Parse(orgaId)

	userOrgList := make([]model.UserOrganization, 0)
	for _, groupUuid := range groups {
		userOrg := model.UserOrganization{
			UserId:         userUuid,
			OrganizationId: orgaUuid,
			GroupId:        groupUuid,
		}

		userOrgList = append(userOrgList, userOrg)
	}

	result := db.Client.Where(model.UserOrganization{
		UserId:         userUuid,
		OrganizationId: orgaUuid,
	}).Delete(model.UserOrganization{})

	if result.Error != nil {
		log.Err(result.Error).Msg("failed to remove previous groups")
		return []model.UserOrganization{}, result.Error
	}

	result = db.Client.Save(&userOrgList)

	if result.Error != nil {
		log.Err(result.Error).Msg("failed to update groups")
		return []model.UserOrganization{}, result.Error
	}
	return userOrgList, nil
}

func RemoveUserFromOrg(ctx context.Context, orgaId, userToRemove string) error {
	log := logger.GetLogger(ctx)

	userUuid, _ := uuid.Parse(userToRemove)
	orgaUuid, _ := uuid.Parse(orgaId)

	result := db.Client.Where(model.UserOrganization{
		UserId:         userUuid,
		OrganizationId: orgaUuid,
	}).Delete(model.UserOrganization{})

	if result.Error != nil {
		log.Err(result.Error).Msg("failed to delete")
		return result.Error
	}

	return nil
}

// IsQuotaCreationReached compare quota with number of owned organization
// Return if the quota is reached
func IsQuotaCreationReached(ctx context.Context, userUuid uuid.UUID) (bool, error) {
	log := logger.GetLogger(ctx)
	userQuota, err := quota.GetQuotaValue(quota.UserLimitOrganization, userUuid, quota.MapQuotaEntityType[quota.UserLimitOrganization])
	if err != nil {
		log.Err(err).Any("userId", userUuid).Msg("failed to get quota")
		return false, err
	}

	count, err := CountOrgaByUser(userUuid)
	if err != nil {
		log.Err(err).Any("userId", userUuid).Msg("failed to count organization")
		return false, err
	}

	orgaLimit, err := strconv.ParseInt(userQuota.Value, 10, 64)
	if err != nil {
		log.Err(err).Any("userQuota", userQuota).Msg("failed to parse quota value")
		return false, err
	}

	return count >= orgaLimit, nil
}

func castToAPIOrganization(org model.Organization) (httpModel.APIOrganization, error) {
	var cast httpModel.APIOrganization
	temporaryVariable, err := json.Marshal(org)
	err = json.Unmarshal(temporaryVariable, &cast)

	if err == nil {
		cast.Users = convertUserAndRoleToUserRole(org.Users, org.UserRoles)
	}
	return cast, err
}

func convertUserAndRoleToUserRole(users []*model.User, roles []*model.UserOrganization) []*httpModel.APIUserGroup {
	var result []*httpModel.APIUserGroup
	// As we have one line by role for each user, we potentially have duplicated users
	// We want to remove them
	seen := make(map[uuid.UUID]bool, len(users))
	for _, user := range users {
		if !seen[user.ID] {
			groups := make([]uuid.UUID, 0)

			for _, role := range roles {
				if user.ID == role.UserId {
					groups = append(groups, role.GroupId)
				}
			}

			result = append(result, &httpModel.APIUserGroup{
				User: &httpModel.APIUserReduce{
					APIModel:   httpModel.APIModel{ID: user.ID},
					Firstname:  user.Firstname,
					Lastname:   user.Lastname,
					Email:      user.Email,
					InviteCode: user.InviteCode,
				},
				Groups: groups,
			})
			seen[user.ID] = true
		}
	}
	return result
}
