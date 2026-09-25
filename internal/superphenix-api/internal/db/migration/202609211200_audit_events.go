package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// migration202609211200 creates audit_events and adds organizations.audit_retention_days.
var migration202609211200 = &gormigrate.Migration{
	ID: "202609211200_audit_events",
	Migrate: func(tx *gorm.DB) error {
		statements := []string{
			`CREATE TABLE IF NOT EXISTS audit_events (
				id uuid NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
				organization_id uuid,
				project_id uuid,
				event_type text NOT NULL,
				resource_type text NOT NULL,
				resource_id text,
				user_id uuid,
				user_email text,
				auth_type text,
				source_ip text,
				remote_addr text,
				status text NOT NULL,
				status_code bigint,
				request_id text,
				started_at timestamptz NOT NULL,
				completed_at timestamptz
			)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_events_org_started
				ON audit_events (organization_id, started_at DESC, id DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_events_started ON audit_events (started_at)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_events_user_started
				ON audit_events (user_id, started_at DESC) WHERE organization_id IS NULL`,
			`ALTER TABLE organizations ADD COLUMN IF NOT EXISTS audit_retention_days bigint`,
		}

		for _, statement := range statements {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202609211200")

		if err := tx.Exec("ALTER TABLE organizations DROP COLUMN IF EXISTS audit_retention_days").Error; err != nil {
			return err
		}

		return tx.Exec("DROP TABLE IF EXISTS audit_events").Error
	},
}
