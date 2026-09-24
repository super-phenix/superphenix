package billing

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
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
		_ = sqlDB.Close()
	}
}

const (
	invalidUUIDTestName = "invalid uuid param returns 400"
	successTestName     = "success"
)

func TestGetResourceName(t *testing.T) {
	validUUID := uuid.New()
	prefixedResourceID := fmt.Sprintf("%s-%s", spxId.FrameworkPrefix(), validUUID.String())

	tests := []struct {
		name           string
		resourceID     string
		mockSetup      func(mock sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:       successTestName,
			resourceID: validUUID.String(),
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "product_name"}).
					AddRow(validUUID, "resource-alpha")
				mock.ExpectQuery(`SELECT .* FROM "products"`).
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "resource-alpha",
		},
		{
			name:           "prefixed id returns 400",
			resourceID:     prefixedResourceID,
			mockSetup:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   consts.SpxWrongPathParam,
		},
		{
			name:           invalidUUIDTestName,
			resourceID:     "not-a-valid-uuid",
			mockSetup:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   consts.SpxWrongPathParam,
		},
		{
			name:       "resource not found returns 404",
			resourceID: validUUID.String(),
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .* FROM "products"`).
					WillReturnError(gorm.ErrRecordNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   consts.SpxResourceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			r := chi.NewRouter()
			svc := New(&config.Global)
			r.Get("/billing/resource/{resourceId}", svc.GetResourceName)

			req := httptest.NewRequest(http.MethodGet, "/billing/resource/"+tt.resourceID, nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.expectedBody)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetProjectName(t *testing.T) {
	validUUID := uuid.New()

	tests := []struct {
		name           string
		projectID      string
		mockSetup      func(mock sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:      successTestName,
			projectID: validUUID.String(),
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name"}).
					AddRow(validUUID, "project-alpha")
				mock.ExpectQuery(`SELECT .* FROM "projects"`).
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "project-alpha",
		},
		{
			name:           invalidUUIDTestName,
			projectID:      "invalid-id",
			mockSetup:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   consts.SpxWrongPathParam,
		},
		{
			name:      "project not found returns 404",
			projectID: validUUID.String(),
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .* FROM "projects"`).
					WillReturnError(gorm.ErrRecordNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   consts.SpxProjectNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			r := chi.NewRouter()
			svc := New(&config.Global)
			r.Get("/billing/project/{projectId}", svc.GetProjectName)

			req := httptest.NewRequest(http.MethodGet, "/billing/project/"+tt.projectID, nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.expectedBody)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetOrgaName(t *testing.T) {
	validUUID := uuid.New()

	tests := []struct {
		name           string
		orgaID         string
		mockSetup      func(mock sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:   successTestName,
			orgaID: validUUID.String(),
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name"}).
					AddRow(validUUID, "orga-alpha")
				mock.ExpectQuery(`SELECT .* FROM "organizations"`).
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "orga-alpha",
		},
		{
			name:           invalidUUIDTestName,
			orgaID:         "invalid-id",
			mockSetup:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   consts.SpxWrongPathParam,
		},
		{
			name:   "orga not found returns 404",
			orgaID: validUUID.String(),
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .* FROM "organizations"`).
					WillReturnError(gorm.ErrRecordNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   consts.SpxOrgNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			r := chi.NewRouter()
			svc := New(&config.Global)
			r.Get("/billing/organization/{orgaId}", svc.GetOrgaName)

			req := httptest.NewRequest(http.MethodGet, "/billing/organization/"+tt.orgaID, nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.expectedBody)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestBillingRoutesRegistration(t *testing.T) {
	reg := router.New()
	cfg := &config.Config{}
	ProvideService(cfg, reg)

	mod := Module(cfg, New(cfg))
	assert.Equal(t, ModuleName, mod.Name)
	assert.Equal(t, "/billing", mod.Mount)

	expectedRoutes := map[string]string{
		"/project/{projectId}":   http.MethodGet,
		"/organization/{orgaId}": http.MethodGet,
		"/resource/{resourceId}": http.MethodGet,
	}

	for path, method := range expectedRoutes {
		found := false
		for _, rt := range mod.Routes {
			if rt.Pattern == path && rt.Method == method {
				found = true
				break
			}
		}
		assert.True(t, found, "route %s %s should be registered", method, path)
	}
}
