package migration

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var migration202510201558 = &gormigrate.Migration{
	ID: "202510201558_replace_old_quota",
	Migrate: func(tx *gorm.DB) error {
		type Quota struct {
			ID string `gorm:"primaryKey;not null"`
			model.TracingModel
			Value string `gorm:"not null"`
		}

		// Remove all older values
		if err := tx.Exec("DELETE FROM quota").Error; err != nil {
			return err
		}

		if err := tx.Migrator().DropColumn("quota", "entity_type"); err != nil {
			return err
		}

		userLimitOrganization := "User_Limit_Organization"
		orgaLimitProject := "Orga_Limit_Project"
		orgaLimitIAMGroup := "Orga_Limit_IAMGroup"
		projectLimitProduct := "Project_Limit_Product"

		var quotas = []Quota{
			{
				ID:    userLimitOrganization,
				Value: "5",
			}, {
				ID:    orgaLimitProject,
				Value: "10",
			}, {
				ID:    orgaLimitIAMGroup,
				Value: "15",
			}, {
				ID:    projectLimitProduct,
				Value: "100",
			},
		}
		if err := tx.CreateInBatches(quotas, 4).Error; err != nil {
			return err
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202510201558")

		type Quota struct {
			ID string `gorm:"primaryKey;not null"`
			model.TracingModel
			EntityType *string
			Value      string `gorm:"not null"`
		}

		// Remove all newer values
		if err := tx.Exec("DELETE FROM quota").Error; err != nil {
			return err
		}

		// Check column exists
		if !tx.Migrator().HasColumn("quota", "entity_type") {
			if err := tx.Migrator().AddColumn(&Quota{}, "entity_type"); err != nil {
				return err
			}
		}

		userEntity := "USER"
		var quotas = []Quota{
			{
				ID:         "LIMIT_ORGANIZATION",
				EntityType: &userEntity,
				Value:      "5",
			},
		}

		if err := tx.Create(quotas).Error; err != nil {
			return err
		}

		return nil
	},
}
