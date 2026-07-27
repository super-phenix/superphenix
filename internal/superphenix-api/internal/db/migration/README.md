# Migration with Gormigrate

[Gormigrate]() is a go module to handle versionned gorm migration.

_**Note:**  
The rollback function is not called automatically in this gormigrate version (an issue is opened).  
This leads to activating transaction migration to use the rollback function from postgreSQL.  
To avoid rollback problems, try to regroup all modification in a single migration file
._

## How it works

The first time gormigrate runned it will execute the `init_schema.go` script and create a `migration` table.  
This `migration` table will be filled with `INIT_SCHEMA` and each migration registered at the time of the first run.  
After, each time a new migration is registered, it will be executed and added to the `migration` table.  

## How to add a migration

To add a migration :

### Create a migration file

The filename should be : `yyyyMMddHHmm_migration_name.go`  
example: *202504171110_create_region_table*

Here is a [Content Template](#template).  

The `ID` should match the filename. This ID will be stored in `migrations` table.  
The `Migrate` function will be executed on the startup.  
The `Rollback` will be executed if migration failed *(not currently used but in case of failure, we need to now how to undo migration manually)*.

### Register the migration

To register the migration, update the `migration.go` file to add `registerMigration(migration)` in function `RunMigration`.

If you register multiple migration file, they will be executed in sequence in the same order as you registered it.  

## Template

```go
var migrationName = &gormigrate.Migration{
	ID: "migration_file_name",
	Migrate: func(tx *gorm.DB) error {},
	Rollback: func(tx *gorm.DB) error {},
}
```


### Example

```go
var migration202504171110 = &gormigrate.Migration{
	ID: "202504171110_create_region_table",
	Migrate: func(tx *gorm.DB) error {
		// it's a good pratice to copy the struct inside the function,
		// so side effects are prevented if the original struct changes during the time
		type region struct {
			Code string `gorm:"primaryKey; not null;uniqueIndex"`
			model.TracingModel
			Name string `gorm:"not null"`
		}

		if err := tx.Migrator().CreateTable(&region{}); err != nil {
			return err
		}

		// it's a good pratice to copy the struct inside the function,
		// so side effects are prevented if the original struct changes during the time
		if err := tx.Migrator().RenameColumn("azs", "region", "code_region"); err != nil {
			return err
		}
		if err := tx.Exec("ALTER TABLE azs ADD CONSTRAINT fk_azs_regions FOREIGN KEY (code_region) REFERENCES regions (code)").Error; err != nil {
			return err
		}

		return nil
	},
	Rollback: func(tx *gorm.DB) error {
		log.Info().Msgf("rolling back migration 20250417110")

		if tx.Migrator().HasConstraint("azs", "fk_regions_azs") {
			if err := tx.Migrator().DropConstraint("azs", "fk_regions_azs"); err != nil {
				return err
			}
		}

		// Check column exists
		if tx.Migrator().HasColumn("azs", "code_region") {
			if err := tx.Migrator().RenameColumn("azs", "code_region", "region"); err != nil {
				return err
			}
		}

		if err := tx.Migrator().DropTable("regions"); err != nil {
			return err
		}
		return nil
	},
}
```