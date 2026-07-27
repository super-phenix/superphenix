package migration

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var migration202606011530 = &gormigrate.Migration{
	ID: "202606011530_user_add_invite_code_regenerated_at",
	Migrate: func(tx *gorm.DB) error {
		type User struct {
			InviteCodeRegeneratedAt *time.Time `gorm:"default:null"`
		}

		if err := tx.Migrator().AddColumn(&User{}, "InviteCodeRegeneratedAt"); err != nil {
			return err
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 202606011530")
		type User struct {
			InviteCodeRegeneratedAt *time.Time `gorm:"default:null"`
		}

		if tx.Migrator().HasColumn("users", "invite_code_regenerated_at") {
			if err := tx.Migrator().DropColumn(&User{}, "InviteCodeRegeneratedAt"); err != nil {
				return err
			}
		}

		return nil
	},
}
