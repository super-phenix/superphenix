package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// migration202611021200 replaces the quota_overrides primary key on id alone with (id, entity_id, entity_type).
var migration202611021200 = &gormigrate.Migration{
	ID: "202611021200_quota_override_composite_key",
	Migrate: func(tx *gorm.DB) error {
		return tx.Exec(`ALTER TABLE quota_overrides
			DROP CONSTRAINT IF EXISTS quota_overrides_pkey,
			ADD CONSTRAINT quota_overrides_pkey PRIMARY KEY (id, entity_id, entity_type)`).Error
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202611021200")

		// Fails if several entities override the same quota: those rows must be removed by hand
		// first, since the old key cannot hold them.
		return tx.Exec(`ALTER TABLE quota_overrides
			DROP CONSTRAINT IF EXISTS quota_overrides_pkey,
			ADD CONSTRAINT quota_overrides_pkey PRIMARY KEY (id)`).Error
	},
}
