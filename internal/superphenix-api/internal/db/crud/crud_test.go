package crud

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// testDBModel mirrors a simple DB model for testing generic crud functions.
type testDBModel struct {
	ID   uuid.UUID `gorm:"primaryKey;type:uuid" json:"id"`
	Name string    `json:"name"`
}

func (testDBModel) TableName() string { return "test_models" }

// testAPIModel is the target type for casting.
type testAPIModel struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// setupMockDB creates a sqlmock-backed gorm.DB and returns the mock and a cleanup func.
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

func TestFindAll(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()

	tests := []struct {
		name        string
		filter      testDBModel
		mockSetup   func(mock sqlmock.Sqlmock)
		wantCount   int
		wantErr     bool
		checkResult func(t *testing.T, result []testAPIModel)
	}{
		{
			name:   "returns multiple rows",
			filter: testDBModel{},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name"}).
					AddRow(id1, "Alpha").
					AddRow(id2, "Beta")
				mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
			},
			wantCount: 2,
			checkResult: func(t *testing.T, result []testAPIModel) {
				assert.Equal(t, "Alpha", result[0].Name)
				assert.Equal(t, "Beta", result[1].Name)
			},
		},
		{
			name:   "returns empty slice when no rows",
			filter: testDBModel{},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name"})
				mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
			},
			wantCount: 0,
			checkResult: func(t *testing.T, result []testAPIModel) {
				assert.NotNil(t, result, "result should be non-nil empty slice")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			result, err := FindAll[testDBModel, testAPIModel](tt.filter)
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

func TestFind(t *testing.T) {
	id1 := uuid.New()

	tests := []struct {
		name        string
		filter      testDBModel
		mockSetup   func(mock sqlmock.Sqlmock)
		wantErr     bool
		checkResult func(t *testing.T, result testAPIModel)
	}{
		{
			name:   "returns single row",
			filter: testDBModel{ID: id1},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name"}).
					AddRow(id1, "Alpha")
				mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
			},
			checkResult: func(t *testing.T, result testAPIModel) {
				assert.Equal(t, id1, result.ID)
				assert.Equal(t, "Alpha", result.Name)
			},
		},
		{
			name:   "returns error when no row found",
			filter: testDBModel{ID: uuid.New()},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name"})
				mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			result, err := Find[testDBModel, testAPIModel](tt.filter)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFindUnscoped(t *testing.T) {
	id1 := uuid.New()

	tests := []struct {
		name        string
		filter      testDBModel
		mockSetup   func(mock sqlmock.Sqlmock)
		wantErr     bool
		checkResult func(t *testing.T, result testAPIModel)
	}{
		{
			name:   "returns single row including soft-deleted",
			filter: testDBModel{ID: id1},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name"}).
					AddRow(id1, "Deleted")
				mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
			},
			checkResult: func(t *testing.T, result testAPIModel) {
				assert.Equal(t, id1, result.ID)
				assert.Equal(t, "Deleted", result.Name)
			},
		},
		{
			name:   "returns error when no row found",
			filter: testDBModel{ID: uuid.New()},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name"})
				mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			result, err := FindUnscoped[testDBModel, testAPIModel](tt.filter)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFindAllWhere(t *testing.T) {
	id1 := uuid.New()

	tests := []struct {
		name        string
		object      testDBModel
		where       testDBModel
		mockSetup   func(mock sqlmock.Sqlmock)
		wantCount   int
		wantErr     bool
		checkResult func(t *testing.T, result []testAPIModel)
	}{
		{
			name:   "returns filtered rows",
			object: testDBModel{},
			where:  testDBModel{Name: "Alpha"},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name"}).
					AddRow(id1, "Alpha")
				mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
			},
			wantCount: 1,
			checkResult: func(t *testing.T, result []testAPIModel) {
				assert.Equal(t, "Alpha", result[0].Name)
			},
		},
		{
			name:   "returns empty slice when no match",
			object: testDBModel{},
			where:  testDBModel{Name: "NonExistent"},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name"})
				mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
			},
			wantCount: 0,
			checkResult: func(t *testing.T, result []testAPIModel) {
				assert.NotNil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			result, err := FindAllWhere[testDBModel, testAPIModel](tt.object, tt.where)
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

func TestFindWhere(t *testing.T) {
	id1 := uuid.New()

	tests := []struct {
		name        string
		object      testDBModel
		where       testDBModel
		mockSetup   func(mock sqlmock.Sqlmock)
		wantErr     bool
		checkResult func(t *testing.T, result testAPIModel)
	}{
		{
			name:   "returns single filtered row",
			object: testDBModel{},
			where:  testDBModel{Name: "Alpha"},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name"}).
					AddRow(id1, "Alpha")
				mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
			},
			checkResult: func(t *testing.T, result testAPIModel) {
				assert.Equal(t, id1, result.ID)
				assert.Equal(t, "Alpha", result.Name)
			},
		},
		{
			name:   "returns error when no match",
			object: testDBModel{},
			where:  testDBModel{Name: "NonExistent"},
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name"})
				mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			result, err := FindWhere[testDBModel, testAPIModel](tt.object, tt.where)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
