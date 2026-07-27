package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var (
	migrations []*gormigrate.Migration
)

func registerMigration(migration *gormigrate.Migration) {
	migrations = append(migrations, migration)
}

func RunMigration(db *gorm.DB) error {
	// Append all migration files
	registerMigration(migration202504171110)
	registerMigration(migration202506101240)
	registerMigration(migration202507111338)
	registerMigration(migration202507211745)
	registerMigration(migration202507241521)
	registerMigration(migration202507291521)
	registerMigration(migration202509111449)
	registerMigration(migration202509241050)
	registerMigration(migration202510201558)
	registerMigration(migration202510291113)
	registerMigration(migration202512111730)
	registerMigration(migration202512150838)
	registerMigration(migration202603131040)
	registerMigration(migration202603171727)
	registerMigration(migration202605271126)
	registerMigration(migration202606011530)
	registerMigration(migration202606021000)
	registerMigration(migration202606291100)

	// Run migrations
	m := gormigrate.New(db, &gormigrate.Options{
		TableName:                 "migrations", // default
		IDColumnName:              "id",         // default
		IDColumnSize:              255,          // default
		UseTransaction:            true,         // changed to true to enable auto rollback if migration failed
		ValidateUnknownMigrations: false,        // default
	}, migrations)
	m.InitSchema(initSchema)

	return m.Migrate()
}
