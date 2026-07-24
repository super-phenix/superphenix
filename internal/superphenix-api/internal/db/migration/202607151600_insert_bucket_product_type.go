package migration

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var migration202607151600 = &gormigrate.Migration{
	ID: "202607151600_insert_bucket_product_type",
	Migrate: func(tx *gorm.DB) error {
		res := tx.Create(&model.ProductType{ID: "bucket", Name: "Bucket"})

		if res.Error != nil {
			return res.Error
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202607151600")

		res := tx.Delete(&model.ProductType{ID: "bucket", Name: "Bucket"})
		if res.Error != nil {
			return res.Error
		}

		return nil
	},
}
