package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var migration202509241050 = &gormigrate.Migration{
	ID: "202509241050_az_new_column_whitelist",
	Migrate: func(tx *gorm.DB) error {
		type AZ struct {
			Whitelist []string `gorm:"serializer:json"`
		}

		if err := tx.Migrator().AddColumn(&AZ{}, "Whitelist"); err != nil {
			return err
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202509241050")
		type AZ struct {
			Whitelist []string `gorm:"serializer:json"`
		}

		// Check column exists
		if tx.Migrator().HasColumn("azs", "whitelist") {
			if err := tx.Migrator().DropColumn(&AZ{}, "Whitelist"); err != nil {
				return err
			}
		}

		return nil
	},
}
