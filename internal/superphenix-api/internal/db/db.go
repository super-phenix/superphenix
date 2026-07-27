package db

import (
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/migration"

	"github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	Client *gorm.DB
)

const (
	KratosProvider = "Kratos"
)

// InitDatabase initialize connection with postgres database
// and call AutoMigrate to update database model
func InitDatabase(host, user, password, dbname, port string) error {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s", host, user, password, dbname, port)
	var err error
	Client, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}

	log.Info().Msg("Successfully connected to database")
	log.Info().Msg("Starting migration")
	if err := migration.RunMigration(Client); err != nil {
		return err
	}
	log.Info().Msg("Migration complete")
	return nil
}
