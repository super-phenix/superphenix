package migration

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var migration202610161200 = &gormigrate.Migration{
	ID: "202610161200_insert_dbaas_product_type",
	Migrate: func(tx *gorm.DB) error {
		res := tx.Create(&model.ProductType{ID: "dbaas", Name: "DBaaS"})

		if res.Error != nil {
			return res.Error
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202610161200")

		res := tx.Delete(&model.ProductType{ID: "dbaas", Name: "DBaaS"})
		if res.Error != nil {
			return res.Error
		}

		return nil
	},
}
