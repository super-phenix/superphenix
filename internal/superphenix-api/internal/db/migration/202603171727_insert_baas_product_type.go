package migration

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var migration202603171727 = &gormigrate.Migration{
	ID: "202603171727_insert_baas_product_type",
	Migrate: func(tx *gorm.DB) error {
		res := tx.Create(&model.ProductType{ID: "baas", Name: "BaaS"})

		if res.Error != nil {
			return res.Error
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202603171727")

		res := tx.Delete(&model.ProductType{ID: "baas", Name: "BaaS"})
		if res.Error != nil {
			return res.Error
		}

		return nil
	},
}
