package migration

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var migration202506101240 = &gormigrate.Migration{
	ID: "202506101240_insert_block_product_type",
	Migrate: func(tx *gorm.DB) error {
		res := tx.Create(&model.ProductType{ID: "block", Name: "Block"})

		if res.Error != nil {
			return res.Error
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202506101240")

		res := tx.Delete(&model.ProductType{ID: "block", Name: "Block"})
		if res.Error != nil {
			return res.Error
		}

		return nil
	},
}
