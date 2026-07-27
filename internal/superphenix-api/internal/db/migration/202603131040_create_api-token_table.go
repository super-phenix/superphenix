package migration

import (
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var migration202603131040 = &gormigrate.Migration{
	ID: "202603131040_create_api-token_table",
	Migrate: func(tx *gorm.DB) error {
		type ApiToken struct {
			model.Model

			Name string `gorm:"not null"`

			Prefix string `gorm:"not null"`

			UserId uuid.UUID  `gorm:"type:uuid;not null"`
			User   model.User `gorm:"foreignKey:UserId"`

			ExpiresAt time.Time

			Salt string `gorm:"not null"`

			TokenEncrypted string `gorm:"index;not null"`
		}

		if err := tx.Migrator().CreateTable(&ApiToken{}); err != nil {
			return err
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202603131040")

		if err := tx.Migrator().DropTable("api_tokens"); err != nil {
			return err
		}
		return nil
	},
}
