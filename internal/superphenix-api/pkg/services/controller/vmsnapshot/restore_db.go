package vmsnapshot

import (
	"errors"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errInstanceOfAnotherProject = errors.New("instance belongs to another project")

// registerRestoredInstance makes sure the VM restored from a snapshot has a
// product row in the caller's project.
//
// The row is looked up by effective ID, which is derived from the project and
// the local ID, so it can only match a row of this project. When it is
// missing, a new row is INSERTED with the given local ID: an insert fails on
// an existing primary key instead of overwriting the row, so a local ID taken
// from another tenant's product cannot hijack it.
func registerRestoredInstance(localId, projectId uuid.UUID, effectiveId, name, codeAZ string) error {
	var instance model.Product
	result := db.Client.Unscoped().Where(model.Product{EffectiveID: effectiveId}).First(&instance)

	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		instance = model.Product{
			Model:         model.Model{ID: localId},
			EffectiveID:   effectiveId,
			ProductName:   name,
			CodeAZ:        codeAZ,
			ProjectId:     projectId,
			ProductTypeId: model.ProductTypeInstance.Name,
		}
		return db.Client.Omit(clause.Associations).Create(&instance).Error
	}

	// Defensive: an effective ID of this project cannot belong to another one.
	if instance.ProjectId != projectId {
		return errInstanceOfAnotherProject
	}

	return db.Client.Unscoped().Model(&instance).
		Updates(map[string]interface{}{"deleted_at": nil, "product_name": name}).Error
}
