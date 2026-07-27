package migration

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var migration202507291521 = &gormigrate.Migration{
	ID: "202507291521_rename_block_to_disk",
	Migrate: func(tx *gorm.DB) error {
		// Add new type
		res := tx.Create(&model.ProductType{ID: "disk", Name: "Disk"})
		if res.Error != nil {
			return res.Error
		}

		// Change all product to the new type
		if err := tx.Exec("update products p set product_type_id = 'disk' where p.product_type_id = 'block';").Error; err != nil {
			return err
		}

		// Remove old type
		res = tx.Delete(&model.ProductType{ID: "block", Name: "Block"})
		if res.Error != nil {
			return res.Error
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202507291521")

		res := tx.FirstOrCreate(&model.ProductType{ID: "block", Name: "Block"})
		if res.Error != nil {
			return res.Error
		}

		if err := tx.Exec("update products p set product_type_id = 'block' where p.product_type_id = 'disk';").Error; err != nil {
			return err
		}

		res = tx.Delete(&model.ProductType{ID: "disk", Name: "Disk"})
		if res.Error != nil {
			return res.Error
		}

		return nil
	},
}
