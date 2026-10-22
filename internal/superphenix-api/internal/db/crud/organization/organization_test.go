package organization

import (
	"regexp"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMockDB(t *testing.T) sqlmock.Sqlmock {
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
	t.Cleanup(func() {
		db.Client = oldClient
		_ = sqlDB.Close()
	})
	return mock
}

// Deleting an organization removes its quotas and the deactivations of its projects in the same
// transaction as the organization and its projects.
func TestDeleteRemovesQuotas(t *testing.T) {
	mock := setupMockDB(t)
	orgaID := uuid.New()
	projects := regexp.QuoteMeta(`(SELECT "id" FROM "projects" WHERE orga_id = $1)`)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id" FROM "organizations" WHERE id = $1`)+`.*FOR UPDATE`).
		WithArgs(orgaID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(orgaID))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "project_az_quotas" WHERE project_id IN `) + projects).
		WithArgs(orgaID).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "project_quotas" WHERE organization_id = $1`)).
		WithArgs(orgaID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "organization_quotas" WHERE organization_id = $1`)).
		WithArgs(orgaID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "project_deactivations" WHERE project_id IN `) + projects).
		WithArgs(orgaID).WillReturnResult(sqlmock.NewResult(0, 1))
	// Associations are deleted in no fixed order.
	mock.ExpectExec(`DELETE FROM "user_organizations"|UPDATE "projects" SET "deleted_at"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM "user_organizations"|UPDATE "projects" SET "deleted_at"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "organizations" SET "deleted_at"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(`"groups"`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	assert.NoError(t, Delete(orgaID.String()))
	assert.NoError(t, mock.ExpectationsWereMet())
}
