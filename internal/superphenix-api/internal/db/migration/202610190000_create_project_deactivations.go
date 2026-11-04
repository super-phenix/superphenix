package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// migration202610190000 creates project_deactivations: a project is deactivated while it has
// at least one row (one per reason).
var migration202610190000 = &gormigrate.Migration{
	ID: "202610190000_create_project_deactivations",
	Migrate: func(tx *gorm.DB) error {
		return tx.Exec(`CREATE TABLE IF NOT EXISTS project_deactivations (
			project_id uuid NOT NULL,
			reason text NOT NULL,
			message text,
			created_at timestamptz,
			updated_at timestamptz,
			PRIMARY KEY (project_id, reason)
		)`).Error
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202610190000")

		return tx.Exec("DROP TABLE IF EXISTS project_deactivations").Error
	},
}
