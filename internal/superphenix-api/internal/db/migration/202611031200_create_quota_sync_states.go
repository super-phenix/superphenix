package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// migration202611031200 replaces the quota revisions of the quota rows with one quota revision per
// organization and creates project_az_quota_states, the sync state of the quota of a project in
// one AZ.
var migration202611031200 = &gormigrate.Migration{
	ID: "202611031200_create_quota_sync_states",
	Migrate: func(tx *gorm.DB) error {
		statements := []string{
			`ALTER TABLE organization_quotas DROP COLUMN IF EXISTS revision`,
			`ALTER TABLE project_quotas DROP COLUMN IF EXISTS revision`,
			`ALTER TABLE project_az_quotas DROP COLUMN IF EXISTS revision`,
			`DROP SEQUENCE IF EXISTS quota_revisions_seq`,
			`ALTER TABLE organizations ADD COLUMN IF NOT EXISTS quota_revision bigint NOT NULL DEFAULT 0`,
			`CREATE TABLE IF NOT EXISTS project_az_quota_states (
				project_id uuid NOT NULL,
				code_az text NOT NULL,
				organization_id uuid NOT NULL,
				wanted_revision bigint NOT NULL DEFAULT 0,
				prepared_revision bigint NOT NULL DEFAULT 0,
				applied_revision bigint NOT NULL DEFAULT 0,
				held jsonb,
				due_at timestamptz NOT NULL DEFAULT now(),
				lease_token uuid,
				attempts bigint NOT NULL DEFAULT 0,
				last_error text,
				PRIMARY KEY (project_id, code_az)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_project_az_quota_states_organization_id ON project_az_quota_states (organization_id)`,
			`CREATE INDEX IF NOT EXISTS idx_project_az_quota_states_due_at ON project_az_quota_states (due_at)`,
		}
		return execAll(tx, statements)
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202611031200")

		statements := []string{
			`DROP TABLE IF EXISTS project_az_quota_states`,
			`ALTER TABLE organizations DROP COLUMN IF EXISTS quota_revision`,
			`CREATE SEQUENCE IF NOT EXISTS quota_revisions_seq`,
			`ALTER TABLE organization_quotas ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT nextval('quota_revisions_seq')`,
			`ALTER TABLE project_quotas ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT nextval('quota_revisions_seq')`,
			`ALTER TABLE project_az_quotas ADD COLUMN IF NOT EXISTS revision bigint NOT NULL DEFAULT nextval('quota_revisions_seq')`,
		}
		return execAll(tx, statements)
	},
}

// execAll runs statements in order and stops at the first error.
func execAll(tx *gorm.DB, statements []string) error {
	for _, statement := range statements {
		if err := tx.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
