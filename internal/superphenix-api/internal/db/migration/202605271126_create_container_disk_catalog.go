package migration

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// containerDiskCatalog202605271126 is a local copy of the removed
// model.ContainerDiskCatalog so this migration keeps compiling. The table is
// dropped by migration 202606021000.
type containerDiskCatalog202605271126 struct {
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

func (containerDiskCatalog202605271126) TableName() string { return "container_disk_catalogs" }

var migration202605271126 = &gormigrate.Migration{
	ID: "202605271126_create_container_disk_catalog",
	Migrate: func(tx *gorm.DB) error {
		if err := tx.Migrator().CreateTable(&containerDiskCatalog202605271126{}); err != nil {
			return err
		}

		seed := []containerDiskCatalog202605271126{
			{
				ID:          "windows-virtio-drivers",
				DisplayName: "Windows VirtIO Driver",
				Image:       "quay.io/kubevirt/virtio-container-disk:v1.7.0",
				Bus:         "sata",
				SupportedOS: []string{"windows"},
				Recommended: true,
			},
		}
		if err := tx.CreateInBatches(seed, 10).Error; err != nil {
			return err
		}
		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202605271126")
		if tx.Migrator().HasTable(&containerDiskCatalog202605271126{}) {
			if err := tx.Migrator().DropTable(&containerDiskCatalog202605271126{}); err != nil {
				return err
			}
		}
		return nil
	},
}
