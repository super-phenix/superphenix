package apiToken

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	httpModel "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"
)

func Save(apiToken model.ApiToken) (model.ApiToken, error) {
	res := db.Client.Save(&apiToken)
	return apiToken, res.Error
}

func FindById(tokenId uuid.UUID) (model.ApiToken, error) {
	return crud.Find[model.ApiToken, model.ApiToken](model.ApiToken{
		Model: model.Model{
			ID: tokenId,
		},
	})
}

func FindAllByTokenPrefix(prefix string) ([]model.ApiToken, error) {
	return crud.FindAll[model.ApiToken, model.ApiToken](model.ApiToken{
		Prefix: prefix,
	}, clause.Associations)
}

func DeleteById(tokenId uuid.UUID) error {
	result := db.Client.Delete(&model.ApiToken{}, tokenId)
	return result.Error
}

func FindAllByUser(userId uuid.UUID) ([]httpModel.APIApiToken, error) {
	apiToken := model.ApiToken{
		UserId: userId,
	}

	return crud.FindAll[model.ApiToken, httpModel.APIApiToken](apiToken)
}
