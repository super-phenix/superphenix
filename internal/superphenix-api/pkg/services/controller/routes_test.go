package controller

import (
	"net/http"
	"net/http/httptest"
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
