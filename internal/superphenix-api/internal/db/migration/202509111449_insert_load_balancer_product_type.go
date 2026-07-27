package migration

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var migration202509111449 = &gormigrate.Migration{
	ID: "202509111449_insert_load_balancer_product_type",
	Migrate: func(tx *gorm.DB) error {
		res := tx.Create(&model.ProductType{ID: "loadBalancer", Name: "Load Balancer"})

		if res.Error != nil {
			return res.Error
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202509111449")

		res := tx.Delete(&model.ProductType{ID: "loadBalancer", Name: "Load Balancer"})
		if res.Error != nil {
			return res.Error
		}

		return nil
	},
}
