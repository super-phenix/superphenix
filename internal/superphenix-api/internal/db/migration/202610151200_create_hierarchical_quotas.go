package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// migration202610151200 creates organization_quotas, project_quotas, and project_az_quotas tables.
var migration202610151200 = &gormigrate.Migration{
	ID: "202610151200_create_hierarchical_quotas",
	Migrate: func(tx *gorm.DB) error {
		statements := []string{
			`CREATE TABLE IF NOT EXISTS organization_quotas (
				organization_id uuid NOT NULL PRIMARY KEY,
				created_at timestamptz,
				updated_at timestamptz,
				deleted_at timestamptz,
				resources jsonb NOT NULL DEFAULT '{}'::jsonb
			)`,
			`CREATE INDEX IF NOT EXISTS idx_organization_quotas_deleted_at ON organization_quotas (deleted_at)`,
			`CREATE TABLE IF NOT EXISTS project_quotas (
				project_id uuid NOT NULL PRIMARY KEY,
				organization_id uuid NOT NULL,
				created_at timestamptz,
				updated_at timestamptz,
				deleted_at timestamptz,
				resources jsonb NOT NULL DEFAULT '{}'::jsonb
			)`,
			`CREATE INDEX IF NOT EXISTS idx_project_quotas_organization_id ON project_quotas (organization_id)`,
			`CREATE INDEX IF NOT EXISTS idx_project_quotas_deleted_at ON project_quotas (deleted_at)`,
			`CREATE TABLE IF NOT EXISTS project_az_quotas (
				id uuid NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
				project_id uuid NOT NULL,
				code_az text NOT NULL,
				created_at timestamptz,
				updated_at timestamptz,
				deleted_at timestamptz,
				resources jsonb NOT NULL DEFAULT '{}'::jsonb
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_proj_az ON project_az_quotas (project_id, code_az)`,
			`CREATE INDEX IF NOT EXISTS idx_project_az_quotas_deleted_at ON project_az_quotas (deleted_at)`,
		}

		for _, statement := range statements {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202610151200")

		if err := tx.Exec("DROP TABLE IF EXISTS project_az_quotas").Error; err != nil {
			return err
		}
		if err := tx.Exec("DROP TABLE IF EXISTS project_quotas").Error; err != nil {
			return err
		}
		return tx.Exec("DROP TABLE IF EXISTS organization_quotas").Error
	},
}
