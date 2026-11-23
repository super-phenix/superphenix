package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
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
		_ = sqlDB.Close()
	}
}

func serveWithParams(params map[string]string) (*httptest.ResponseRecorder, bool) {
	called := false
	handler := CheckProjectInOrganization(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec, called
}

var projectLookup = regexp.QuoteMeta(`SELECT * FROM "projects" WHERE (id = $1 AND orga_id = $2)`)

func TestCheckProjectInOrganization(t *testing.T) {
	orgaId := uuid.New()
	projectId := uuid.New()

	t.Run("project of the organization", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()
		mock.ExpectQuery(projectLookup).
			WithArgs(projectId, orgaId, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "orga_id"}).AddRow(projectId, orgaId))

		rec, called := serveWithParams(map[string]string{"orgaId": orgaId.String(), "projectId": projectId.String()})
		assert.True(t, called)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("project of another organization", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()
		mock.ExpectQuery(projectLookup).
			WithArgs(projectId, orgaId, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "orga_id"}))

		rec, called := serveWithParams(map[string]string{"orgaId": orgaId.String(), "projectId": projectId.String()})
		assert.False(t, called)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("invalid ids are refused without a query", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()

		for _, params := range []map[string]string{
			{"orgaId": orgaId.String(), "projectId": "not-a-uuid"},
			{"orgaId": "not-a-uuid", "projectId": projectId.String()},
			{"orgaId": orgaId.String(), "projectId": uuid.Nil.String()},
		} {
			rec, called := serveWithParams(params)
			assert.False(t, called)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		}
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("route without project", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()

		rec, called := serveWithParams(map[string]string{"orgaId": orgaId.String()})
		assert.True(t, called)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSharedMiddlewaresCheckProjectInOrganization(t *testing.T) {
	want := reflect.ValueOf(CheckProjectInOrganization).Pointer()
	for _, mw := range SharedMiddlewares() {
		if reflect.ValueOf(mw).Pointer() == want {
			return
		}
	}
	t.Fatal("CheckProjectInOrganization is not in the shared controller middlewares")
}

func TestCheckProjectActive(t *testing.T) {
	orgID, projectID := uuid.New(), uuid.New()
	sqlDeactivations := regexp.QuoteMeta(`SELECT * FROM "project_deactivations" WHERE project_id = $1 AND project_id IN (SELECT "id" FROM "projects" WHERE orga_id = $2 AND "projects"."deleted_at" IS NULL) ORDER BY reason`)

	setup := func(t *testing.T) (sqlmock.Sqlmock, chi.Router) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
		require.NoError(t, err)
		oldClient := db.Client
		db.Client = gormDB
		t.Cleanup(func() {
			assert.NoError(t, mock.ExpectationsWereMet())
			db.Client = oldClient
			_ = sqlDB.Close()
		})

		r := chi.NewRouter()
		ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
		r.With(CheckProjectActive).Get("/{orgaId}/v1/{az}/{projectId}/disk", ok)
		r.With(CheckProjectActive).Get("/{orgaId}/v1/azs", ok)
		return mock, r
	}
	serve := func(r chi.Router, path string) int {
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		return rr.Code
	}
	projectPath := "/" + orgID.String() + "/v1/az1/" + projectID.String() + "/disk"

	t.Run("active project", func(t *testing.T) {
		mock, r := setup(t)
		mock.ExpectQuery(sqlDeactivations).WithArgs(projectID, orgID).
			WillReturnRows(sqlmock.NewRows([]string{"project_id", "reason"}))
		assert.Equal(t, http.StatusOK, serve(r, projectPath))
	})

	t.Run("deactivated project", func(t *testing.T) {
		mock, r := setup(t)
		mock.ExpectQuery(sqlDeactivations).WithArgs(projectID, orgID).
			WillReturnRows(sqlmock.NewRows([]string{"project_id", "reason", "message"}).AddRow(projectID, "quotaFailedToApply", "quota not applied on: az1"))
		assert.Equal(t, http.StatusLocked, serve(r, projectPath))
	})

	t.Run("route without project", func(t *testing.T) {
		_, r := setup(t)
		assert.Equal(t, http.StatusOK, serve(r, "/"+orgID.String()+"/v1/azs"))
	})

	t.Run("database error", func(t *testing.T) {
		mock, r := setup(t)
		mock.ExpectQuery(sqlDeactivations).WithArgs(projectID, orgID).WillReturnError(assert.AnError)
		assert.Equal(t, http.StatusInternalServerError, serve(r, projectPath))
	})
}
