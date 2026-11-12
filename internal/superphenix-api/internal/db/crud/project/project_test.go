package project

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

// setupMockDB creates a sqlmock-backed gorm.DB and swaps it into db.Client.
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

func TestFindByIdAndOrgaId(t *testing.T) {
	const query = `SELECT * FROM "projects" WHERE (id = $1 AND orga_id = $2) AND "projects"."deleted_at" IS NULL ORDER BY "projects"."id" LIMIT $3`

	projectID := uuid.New()
	orgaID := uuid.New()

	tests := []struct {
		name      string
		projectID uuid.UUID
		orgaID    uuid.UUID
		found     bool
		wantErr   error
	}{
		{
			name:      "project in organization",
			projectID: projectID,
			orgaID:    orgaID,
			found:     true,
		},
		{
			name:      "project in another organization",
			projectID: projectID,
			orgaID:    uuid.New(),
			wantErr:   gorm.ErrRecordNotFound,
		},
		{
			name:      "nil project id keeps the id condition",
			projectID: uuid.Nil,
			orgaID:    orgaID,
			wantErr:   gorm.ErrRecordNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := setupMockDB(t)

			rows := sqlmock.NewRows([]string{"id", "orga_id", "name"})
			if tt.found {
				rows.AddRow(tt.projectID, tt.orgaID, "my-project")
			}
			mock.ExpectQuery(regexp.QuoteMeta(query)).
				WithArgs(tt.projectID, tt.orgaID, 1).
				WillReturnRows(rows)

			p, err := FindByIdAndOrgaId(tt.projectID, tt.orgaID)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.projectID, p.ID)
				assert.Equal(t, tt.orgaID, p.OrgaId)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDeleteById(t *testing.T) {
	const query = `UPDATE "projects" SET "deleted_at"=$1 WHERE (id = $2 AND orga_id = $3) AND "projects"."deleted_at" IS NULL`

	projectID := uuid.New()
	orgaID := uuid.New()

	tests := []struct {
		name         string
		rowsAffected int64
		wantErr      error
	}{
		{name: "project of the organization is deleted", rowsAffected: 1},
		{name: "project of another organization is not deleted", rowsAffected: 0, wantErr: gorm.ErrRecordNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := setupMockDB(t)

			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id" FROM "organizations" WHERE id = $1`)+`.*FOR UPDATE`).
				WithArgs(orgaID, 1).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(orgaID))
			mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "project_az_quotas" WHERE project_id = $1`)).
				WithArgs(projectID).WillReturnResult(sqlmock.NewResult(0, 2))
			mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "project_quotas" WHERE project_id = $1`)).
				WithArgs(projectID).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec(regexp.QuoteMeta(query)).
				WithArgs(sqlmock.AnyArg(), projectID, orgaID).
				WillReturnResult(sqlmock.NewResult(0, tt.rowsAffected))
			if tt.wantErr != nil {
				// Nothing is kept: the quota rows of a project of another organization are restored.
				mock.ExpectRollback()
			} else {
				mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "project_deactivations" WHERE project_id = $1`)).
					WithArgs(projectID).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}

			err := DeleteById(projectID, orgaID)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
