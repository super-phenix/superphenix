package ctrlutils

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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

func TestCheckProductBelongsToProject(t *testing.T) {
	const (
		eid = "spx-product"
		az  = "az1"
	)
	projectId := uuid.New()
	columns := []string{"id", "effective_id", "product_name", "code_az", "project_id", "product_type_id"}
	row := func(project uuid.UUID, azCode, productType string) *sqlmock.Rows {
		return sqlmock.NewRows(columns).AddRow(uuid.New(), eid, "name", azCode, project, productType)
	}

	tests := []struct {
		name     string
		rows     *sqlmock.Rows
		dbErr    error
		want     bool
		wantCode int
	}{
		{name: "no row", rows: sqlmock.NewRows(columns), want: true, wantCode: http.StatusOK},
		{name: "matching row", rows: row(projectId, az, model.ProductTypeDBaaS.Name), want: true, wantCode: http.StatusOK},
		{name: "other project", rows: row(uuid.New(), az, model.ProductTypeDBaaS.Name), wantCode: http.StatusNotFound},
		{name: "other az", rows: row(projectId, "az2", model.ProductTypeDBaaS.Name), wantCode: http.StatusNotFound},
		{name: "other product type", rows: row(projectId, az, model.ProductTypeKaaS.Name), wantCode: http.StatusNotFound},
		{name: "db error", dbErr: errors.New("connection lost"), wantCode: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := setupMockDB(t)
			query := mock.ExpectQuery(`SELECT \* FROM "products"`)
			if tt.dbErr != nil {
				query.WillReturnError(tt.dbErr)
			} else {
				query.WillReturnRows(tt.rows)
			}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("effectiveId", eid)
			r := httptest.NewRequest(http.MethodDelete, "/", http.NoBody)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
			w := httptest.NewRecorder()

			got := CheckProductBelongsToProject(w, r, projectId, az, model.ProductTypeDBaaS.Name)
			if got != tt.want {
				t.Errorf("CheckProductBelongsToProject() = %v, want %v", got, tt.want)
			}
			if w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantCode)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}
