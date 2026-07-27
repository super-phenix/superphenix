package migration

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var migration202512111730 = &gormigrate.Migration{
	ID: "202512111730_insert_kaas_product_type",
	Migrate: func(tx *gorm.DB) error {
		res := tx.Create(&model.ProductType{ID: "kaas", Name: "KaaS"})

		if res.Error != nil {
			return res.Error
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202512111730")

		res := tx.Delete(&model.ProductType{ID: "kaas", Name: "KaaS"})
		if res.Error != nil {
			return res.Error
		}

		return nil
	},
}
