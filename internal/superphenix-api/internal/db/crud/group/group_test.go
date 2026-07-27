package group

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	httpModel "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMockDB(t *testing.T) (sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
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

func TestFindAllByOrgaIdExceptOwner(t *testing.T) {
	orgaId := uuid.New()
	groupId1 := uuid.New()
	groupId2 := uuid.New()

	tests := []struct {
		name        string
		orgaId      string
		mockSetup   func(mock sqlmock.Sqlmock)
		wantCount   int
		wantErr     bool
		checkResult func(t *testing.T, result []httpModel.APIGroup)
	}{
		{
			name:   "returns groups excluding owner",
			orgaId: orgaId.String(),
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "orga_id", "all_projects", "project_ids", "permission_sets"}).
					AddRow(groupId1, "Developers", orgaId, false, "[]", "[]").
					AddRow(groupId2, "Viewers", orgaId, false, "[]", "[]")
				mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
			},
			wantCount: 2,
			checkResult: func(t *testing.T, result []httpModel.APIGroup) {
				assert.Equal(t, "Developers", result[0].Name)
				assert.Equal(t, "Viewers", result[1].Name)
			},
		},
		{
			name:   "returns empty slice when no groups",
			orgaId: orgaId.String(),
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "orga_id", "all_projects", "project_ids", "permission_sets"})
				mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
			},
			wantCount: 0,
			checkResult: func(t *testing.T, result []httpModel.APIGroup) {
				assert.NotNil(t, result, "result should be non-nil empty slice")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			result, err := FindAllByOrgaIdExceptOwner(tt.orgaId)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Len(t, result, tt.wantCount)
			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
