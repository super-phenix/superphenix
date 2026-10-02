package vmsnapshot

import (
	"errors"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMockDB(t *testing.T) (sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm: %v", err)
	}
	oldClient := db.Client
	db.Client = gormDB
	return mock, func() {
		db.Client = oldClient
		sqlDB.Close()
	}
}

var productColumns = []string{"id", "effective_id", "product_name", "code_az", "project_id", "product_type_id"}

// TestRegisterRestoredInstance checks that a missing instance is INSERTED:
// sqlmock fails on any unexpected statement, so an UPDATE of an existing row
// (the former GORM Save behavior) makes these tests fail.
func TestRegisterRestoredInstance(t *testing.T) {
	projectId := uuid.New()
	localId := uuid.New()
	const eid = "spx-aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	t.Run("missing instance is inserted, never updated", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()

		mock.ExpectQuery(`SELECT \* FROM "products"`).WillReturnRows(sqlmock.NewRows(productColumns))
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "products"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(localId))
		mock.ExpectCommit()

		if err := registerRestoredInstance(localId, projectId, eid, "vm", "az1"); err != nil {
			t.Fatalf("registerRestoredInstance() error = %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
	})

	t.Run("local ID of an existing product fails instead of overwriting it", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()

		mock.ExpectQuery(`SELECT \* FROM "products"`).WillReturnRows(sqlmock.NewRows(productColumns))
		mock.ExpectBegin()
		mock.ExpectQuery(`INSERT INTO "products"`).WillReturnError(errors.New(`duplicate key value violates unique constraint "products_pkey"`))
		mock.ExpectRollback()

		if err := registerRestoredInstance(localId, projectId, eid, "vm", "az1"); err == nil {
			t.Fatal("expected an error on primary key conflict")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
	})

	t.Run("existing instance of the project is restored", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()

		mock.ExpectQuery(`SELECT \* FROM "products"`).WillReturnRows(
			sqlmock.NewRows(productColumns).AddRow(localId, eid, "old", "az1", projectId, "instance"))
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE "products"`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		if err := registerRestoredInstance(localId, projectId, eid, "vm", "az1"); err != nil {
			t.Fatalf("registerRestoredInstance() error = %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
	})

	t.Run("instance of another project is never updated", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()

		mock.ExpectQuery(`SELECT \* FROM "products"`).WillReturnRows(
			sqlmock.NewRows(productColumns).AddRow(localId, eid, "old", "az1", uuid.New(), "instance"))

		if err := registerRestoredInstance(localId, projectId, eid, "vm", "az1"); !errors.Is(err, errInstanceOfAnotherProject) {
			t.Fatalf("expected errInstanceOfAnotherProject, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
	})
}
