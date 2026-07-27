package quota

import (
	"errors"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func GetDefaultQuota(id string) (model.Quota, error) {
	return crud.Find[model.Quota, model.Quota](model.Quota{
		ID: id,
	})
}

// GetQuotaValue Return the value for a specific quota
// If an override is set, then return it, else return the default value
// Return an error if the quota doesn't exist
func GetQuotaValue(id string, entityId uuid.UUID, entityType string) (model.Quota, error) {
	userQuota, err := crud.Find[model.QuotaOverride, model.Quota](model.QuotaOverride{
		ID:         id,
		EntityId:   entityId,
		EntityType: entityType,
	})

	// If not found, try to find default value
	if errors.Is(err, gorm.ErrRecordNotFound) {
		defaultQuota, err := GetDefaultQuota(id)
		return defaultQuota, err
	}

	return userQuota, err
}
