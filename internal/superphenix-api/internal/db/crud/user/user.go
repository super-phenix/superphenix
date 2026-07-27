package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	httpModel "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func FindByProviderId(providerId, provider string) (httpModel.APIUser, error) {
	return crud.Find[model.User, httpModel.APIUser](model.User{
		Provider:   provider,
		ProviderId: providerId,
	})
}

func FindById(userId string) (httpModel.APIUser, error) {
	userUuid, _ := uuid.Parse(userId)
	return crud.Find[model.User, httpModel.APIUser](model.User{
		Model: model.Model{
			ID: userUuid,
		},
	})
}

func FindWithOrganization(userId string) (httpModel.APIUser, error) {
	userUuid, _ := uuid.Parse(userId)
	user := model.User{
		Model: model.Model{
			ID: userUuid,
		},
	}
	return fetchUserWithOrganization(user)
}

func FindWithOrganizationByProvider(providerId, provider string) (httpModel.APIUser, error) {
	user := model.User{
		Provider:   provider,
		ProviderId: providerId,
	}
	return fetchUserWithOrganization(user)
}

func fetchUserWithOrganization(where model.User) (httpModel.APIUser, error) {
	u := &model.User{}
	var cast httpModel.APIUser
	if err := db.Client.
		Where(&where).
		Preload("PersonalOrg").
		First(u).Error; err != nil {
		return cast, err
	}

	if err := db.Client.Model(&model.Organization{}).
		Distinct("organizations.id", "organizations.name", "organizations.owner_id").
		Joins("JOIN user_organizations uo ON uo.organization_id = organizations.id").
		Where("uo.user_id = ?", u.ID).
		Find(&u.GuestOrg).Error; err != nil {
		return cast, err
	}

	raw, err := json.Marshal(u)
	if err != nil {
		return cast, err
	}
	if err := json.Unmarshal(raw, &cast); err != nil {
		return cast, err
	}

	return cast, nil

}

func FindByInviteCode(inviteCode uuid.UUID) (model.User, error) {
	return crud.Find[model.User, model.User](model.User{
		InviteCode: inviteCode,
	})
}

var ErrInviteCodeCooldown = fmt.Errorf("invite code regeneration cooldown has not elapsed")

func RegenerateInviteCode(userId uuid.UUID) (uuid.UUID, error) {
	cooldown := config.Global.UserSettings.InviteCodeRegenerationCooldown

	var u model.User
	if err := db.Client.Select("invite_code_regenerated_at").Where("id = ?", userId).First(&u).Error; err != nil {
		return uuid.Nil, err
	}

	if u.InviteCodeRegeneratedAt != nil && time.Since(*u.InviteCodeRegeneratedAt) < cooldown {
		return uuid.Nil, ErrInviteCodeCooldown
	}

	now := time.Now()
	newCode := uuid.New()
	result := db.Client.Model(&model.User{}).Where("id = ?", userId).Updates(map[string]interface{}{
		"invite_code":                newCode,
		"invite_code_regenerated_at": now,
	})
	if result.Error != nil {
		return uuid.Nil, result.Error
	}
	if result.RowsAffected == 0 {
		return uuid.Nil, gorm.ErrRecordNotFound
	}
	return newCode, nil
}

func IsUserActive(userId string) (bool, error) {
	userUuid, _ := uuid.Parse(userId)
	user, err := crud.Find[model.User, model.User](model.User{
		Model: model.Model{
			ID: userUuid,
		},
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		} else {
			return false, err
		}
	}
	return user.IsActive, nil

}
