package migration

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// containerDiskCatalog202606021000 mirrors the dropped table for the rollback.
type containerDiskCatalog202606021000 struct {
	ID          string `gorm:"primaryKey;not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	DisplayName string         `gorm:"not null"`
	Image       string         `gorm:"not null"`
	Bus         string         `gorm:"not null"`
	SupportedOS []string       `gorm:"serializer:json"`
	Recommended bool           `gorm:"not null;default:false"`
}

func (containerDiskCatalog202606021000) TableName() string { return "container_disk_catalogs" }

var migration202606021000 = &gormigrate.Migration{
	ID: "202606021000_drop_container_disk_catalog",
	Migrate: func(tx *gorm.DB) error {
		log.Info().Msg("dropping container_disk_catalogs table (catalog moved to AZ controller config)")
		if tx.Migrator().HasTable(&containerDiskCatalog202606021000{}) {
			if err := tx.Migrator().DropTable(&containerDiskCatalog202606021000{}); err != nil {
				return err
			}
		}
		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msg("rolling back migration 202606021000 - recreating empty container_disk_catalogs table")
		if !tx.Migrator().HasTable(&containerDiskCatalog202606021000{}) {
			return tx.Migrator().CreateTable(&containerDiskCatalog202606021000{})
		}
		return nil
	},
}
