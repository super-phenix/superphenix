package migration

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var migration202504171110 = &gormigrate.Migration{
	ID: "202504171110_create_region_table",
	Migrate: func(tx *gorm.DB) error {
		// it's a good practice to copy the struct inside the function, so side effects are prevented if the original struct changes during the time
		type region struct {
			Code string `gorm:"primaryKey; not null;uniqueIndex"`
			model.TracingModel
			Name string `gorm:"not null"`
		}

		if err := tx.Migrator().CreateTable(&region{}); err != nil {
			return err
		}

		// it's a good practice to copy the struct inside the function, so side effects are prevented if the original struct changes during the time
		if err := tx.Migrator().RenameColumn("azs", "region", "code_region"); err != nil {
			return err
		}
		if err := tx.Exec("ALTER TABLE azs ADD CONSTRAINT fk_azs_regions FOREIGN KEY (code_region) REFERENCES regions (code)").Error; err != nil {
			return err
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 20250417110")

		if tx.Migrator().HasConstraint("azs", "fk_regions_azs") {
			if err := tx.Migrator().DropConstraint("azs", "fk_regions_azs"); err != nil {
				return err
			}
		}

		// Check column exists
		if tx.Migrator().HasColumn("azs", "code_region") {
			if err := tx.Migrator().RenameColumn("azs", "code_region", "region"); err != nil {
				return err
			}
		}

		if err := tx.Migrator().DropTable("regions"); err != nil {
			return err
		}
		return nil
	},
}
