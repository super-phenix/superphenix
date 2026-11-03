package product

import (
	"errors"
	"regexp"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var accessibleAZs = []string{"az1", "az2"}

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
		_ = sqlDB.Close()
	}
}

func TestCountByTypeForProject(t *testing.T) {
	projectId := uuid.New()

	tests := []struct {
		name         string
		productTypes []string
		azCodes      []string
		mockSetup    func(mock sqlmock.Sqlmock)
		want         map[string]int64
		wantErr      bool
	}{
		{
			name:         "counts the requested types",
			productTypes: []string{model.ProductTypeInstance.Name, model.ProductTypeDisk.Name},
			azCodes:      accessibleAZs,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"product_type_id", "count"}).
					AddRow(model.ProductTypeInstance.Name, 3).
					AddRow(model.ProductTypeDisk.Name, 5)
				mock.ExpectQuery(`SELECT product_type_id, count\(\*\) as count FROM "products" WHERE .*code_az IN \(\$\d+,\$\d+\)`).
					WillReturnRows(rows)
			},
			want: map[string]int64{
				model.ProductTypeInstance.Name: 3,
				model.ProductTypeDisk.Name:     5,
			},
		},
		{
			name:         "a requested type with no row counts zero",
			productTypes: []string{model.ProductTypeInstance.Name, model.ProductTypeKaaS.Name},
			azCodes:      accessibleAZs,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"product_type_id", "count"}).
					AddRow(model.ProductTypeInstance.Name, 2)
				mock.ExpectQuery(`SELECT product_type_id, count\(\*\) as count FROM "products"`).
					WillReturnRows(rows)
			},
			want: map[string]int64{
				model.ProductTypeInstance.Name: 2,
				model.ProductTypeKaaS.Name:     0,
			},
		},
		{
			name:         "no accessible AZ counts zero without querying",
			productTypes: []string{model.ProductTypeInstance.Name},
			azCodes:      []string{},
			mockSetup:    func(_ sqlmock.Sqlmock) {},
			want:         map[string]int64{model.ProductTypeInstance.Name: 0},
		},
		{
			name:         "no requested type does not query",
			productTypes: []string{},
			azCodes:      accessibleAZs,
			mockSetup:    func(_ sqlmock.Sqlmock) {},
			want:         map[string]int64{},
		},
		{
			name:         "propagates the query error",
			productTypes: []string{model.ProductTypeInstance.Name},
			azCodes:      accessibleAZs,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT product_type_id, count\(\*\) as count FROM "products"`).
					WillReturnError(errors.New("boom"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			got, err := CountByTypeForProject(projectId, tt.productTypes, tt.azCodes)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCountByAZForProject(t *testing.T) {
	projectId := uuid.New()

	tests := []struct {
		name         string
		productTypes []string
		azCodes      []string
		mockSetup    func(mock sqlmock.Sqlmock)
		want         map[string]int64
		wantErr      bool
	}{
		{
			name:         "groups the counts by AZ",
			productTypes: []string{model.ProductTypeInstance.Name, model.ProductTypeDisk.Name},
			azCodes:      accessibleAZs,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"code_az", "count"}).
					AddRow("az1", 4).
					AddRow("az2", 2)
				mock.ExpectQuery(`SELECT code_az, count\(\*\) as count FROM "products" WHERE .*code_az IN \(\$\d+,\$\d+\)`).
					WillReturnRows(rows)
			},
			want: map[string]int64{"az1": 4, "az2": 2},
		},
		{
			name:         "an AZ without product is absent",
			productTypes: []string{model.ProductTypeInstance.Name},
			azCodes:      accessibleAZs,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"code_az", "count"})
				mock.ExpectQuery(`SELECT code_az, count\(\*\) as count FROM "products"`).
					WillReturnRows(rows)
			},
			want: map[string]int64{},
		},
		{
			name:         "no accessible AZ returns no count without querying",
			productTypes: []string{model.ProductTypeInstance.Name},
			azCodes:      []string{},
			mockSetup:    func(_ sqlmock.Sqlmock) {},
			want:         map[string]int64{},
		},
		{
			name:         "no readable type does not query",
			productTypes: []string{},
			azCodes:      accessibleAZs,
			mockSetup:    func(_ sqlmock.Sqlmock) {},
			want:         map[string]int64{},
		},
		{
			name:         "propagates the query error",
			productTypes: []string{model.ProductTypeInstance.Name},
			azCodes:      accessibleAZs,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT code_az, count\(\*\) as count FROM "products"`).
					WillReturnError(errors.New("boom"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			got, err := CountByAZForProject(projectId, tt.productTypes, tt.azCodes)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFindLastCreatedByProject(t *testing.T) {
	projectId := uuid.New()
	firstId := uuid.New()
	secondId := uuid.New()

	tests := []struct {
		name         string
		productTypes []string
		azCodes      []string
		limit        int
		mockSetup    func(mock sqlmock.Sqlmock)
		wantNames    []string
		wantErr      bool
	}{
		{
			name:         "returns the products newest first",
			productTypes: []string{model.ProductTypeInstance.Name},
			azCodes:      accessibleAZs,
			limit:        5,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "product_name", "product_type_id", "code_az"}).
					AddRow(firstId, "web-01", model.ProductTypeInstance.Name, "az1").
					AddRow(secondId, "web-02", model.ProductTypeInstance.Name, "az2")
				mock.ExpectQuery(`SELECT \* FROM "products" WHERE .*code_az IN \(\$\d+,\$\d+\)`).WillReturnRows(rows)
			},
			wantNames: []string{"web-01", "web-02"},
		},
		{
			name:         "no readable type does not query",
			productTypes: []string{},
			azCodes:      accessibleAZs,
			limit:        5,
			mockSetup:    func(_ sqlmock.Sqlmock) {},
			wantNames:    []string{},
		},
		{
			name:         "no accessible AZ returns no product without querying",
			productTypes: []string{model.ProductTypeInstance.Name},
			azCodes:      []string{},
			limit:        5,
			mockSetup:    func(_ sqlmock.Sqlmock) {},
			wantNames:    []string{},
		},
		{
			name:         "a non-positive limit does not query",
			productTypes: []string{model.ProductTypeInstance.Name},
			azCodes:      accessibleAZs,
			limit:        0,
			mockSetup:    func(_ sqlmock.Sqlmock) {},
			wantNames:    []string{},
		},
		{
			name:         "propagates the query error",
			productTypes: []string{model.ProductTypeInstance.Name},
			azCodes:      accessibleAZs,
			limit:        5,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "products"`).WillReturnError(errors.New("boom"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			got, err := FindLastCreatedByProject(projectId, tt.productTypes, tt.azCodes, tt.limit)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				names := make([]string, 0, len(got))
				for _, p := range got {
					names = append(names, p.ProductName)
				}
				assert.Equal(t, tt.wantNames, names)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCountForProject(t *testing.T) {
	projectId := uuid.New()

	tests := []struct {
		name      string
		mockSetup func(mock sqlmock.Sqlmock)
		want      int64
		wantErr   bool
	}{
		{
			name: "counts every product type",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(42)
				mock.ExpectQuery(`SELECT count\(\*\) FROM "products"`).WillReturnRows(rows)
			},
			want: 42,
		},
		{
			name: "propagates the query error",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT count\(\*\) FROM "products"`).WillReturnError(errors.New("boom"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			got, err := CountForProject(projectId)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestDeleteByProject checks that products are only deleted through a
// subquery scoped to the project AND its organization.
func TestDeleteByProject(t *testing.T) {
	const query = `UPDATE "products" SET "deleted_at"=$1 WHERE project_id IN (SELECT "id" FROM "projects" WHERE (id = $2 AND orga_id = $3) AND "projects"."deleted_at" IS NULL) AND "products"."deleted_at" IS NULL`

	mock, cleanup := setupMockDB(t)
	defer cleanup()

	projectId := uuid.New()
	orgaId := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(query)).
		WithArgs(sqlmock.AnyArg(), projectId, orgaId).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectCommit()

	assert.NoError(t, DeleteByProject(projectId, orgaId))
	assert.NoError(t, mock.ExpectationsWereMet())
}
