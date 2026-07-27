package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var migration202512150838 = &gormigrate.Migration{
	ID: "202512150838_az_new_column_destination",
	Migrate: func(tx *gorm.DB) error {
		type AZ struct {
			Destination string
		}

		if err := tx.Migrator().AddColumn(&AZ{}, "Destination"); err != nil {
			return err
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202512150838")
		type AZ struct {
			Destination string
		}

		// Check column exists
		if tx.Migrator().HasColumn("azs", "destination") {
			if err := tx.Migrator().DropColumn(&AZ{}, "Destination"); err != nil {
				return err
			}
		}

		return nil
	},
}
