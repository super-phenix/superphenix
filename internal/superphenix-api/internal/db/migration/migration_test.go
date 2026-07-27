package migration

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRunMigration(t *testing.T) {
	t.Skip("Skipping TestRunMigration - Only run local")
	host := "localhost"
	user := "postgres"
	password := "secret"
	dbname := "user_db"
	port := "5433"
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s", host, user, password, dbname, port)
	var err error
	Client, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Error(err)
		return
	}

	log.Info().Msg("Successfully connected to database")
	log.Info().Msg("Starting migration")
	if err := RunMigration(Client); err != nil {
		t.Error(err)
		return
	}
	log.Info().Msg("Migration complete")
	return

}
