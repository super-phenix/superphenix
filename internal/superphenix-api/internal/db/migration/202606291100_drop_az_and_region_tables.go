package migration

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// region202606291100 mirrors the dropped regions table for the rollback.
type region202606291100 struct {
	Code      string `gorm:"primaryKey;not null;uniqueIndex"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Name      string         `gorm:"not null"`
}

func (region202606291100) TableName() string { return "regions" }

// az202606291100 mirrors the dropped azs table for the rollback.
type az202606291100 struct {
	Code        string `gorm:"primaryKey;not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	Name        string         `gorm:"not null"`
	CodeRegion  string         `gorm:"not null"`
	Url         string         `gorm:"not null"`
	Destination string
	Whitelist   []string `gorm:"serializer:json"`
}

func (az202606291100) TableName() string { return "azs" }

var migration202606291100 = &gormigrate.Migration{
	ID: "202606291100_drop_az_and_region_tables",
	Migrate: func(tx *gorm.DB) error {
		// Drop foreign key constraint from azs to regions first
		if tx.Migrator().HasConstraint("azs", "fk_azs_regions") {
			if err := tx.Migrator().DropConstraint("azs", "fk_azs_regions"); err != nil {
				return err
			}
		}

		if err := tx.Migrator().DropTable(&az202606291100{}); err != nil {
			return err
		}

		if err := tx.Migrator().DropTable(&region202606291100{}); err != nil {
			return err
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msg("rolling back migration 202606291100 - recreating empty regions and azs tables")

		if !tx.Migrator().HasTable(&region202606291100{}) {
			if err := tx.Migrator().CreateTable(&region202606291100{}); err != nil {
				return err
			}
		}

		if !tx.Migrator().HasTable(&az202606291100{}) {
			if err := tx.Migrator().CreateTable(&az202606291100{}); err != nil {
				return err
			}
		}

		// Recreate foreign key constraint
		if err := tx.Exec("ALTER TABLE azs ADD CONSTRAINT fk_azs_regions FOREIGN KEY (code_region) REFERENCES regions (code)").Error; err != nil {
			return err
		}

		return nil
	},
}
