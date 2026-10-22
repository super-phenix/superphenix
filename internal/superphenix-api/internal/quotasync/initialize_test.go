package quotasync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
)

var (
	sqlDeactivate = regexp.QuoteMeta(`INSERT INTO "project_deactivations"`) + `.*ON CONFLICT \("project_id","reason"\) DO UPDATE`
	sqlReactivate = regexp.QuoteMeta(`DELETE FROM "project_deactivations" WHERE project_id = $1 AND reason = $2`)
)

func setupMockDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
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
	return mock
}

func setupController(t *testing.T, status int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	old := config.Global.AZs
	config.Global.AZs = map[string]config.AZConfig{"az1": {Code: "az1", ControllerUrl: srv.URL}}
	t.Cleanup(func() {
		srv.Close()
		config.Global.AZs = old
	})
}

// expectHierarchy expects the reading of the quotas of a project, the organization limiting
// orgResources (none when empty).
func expectHierarchy(mock sqlmock.Sqlmock, orgID uuid.UUID, orgResources string) {
	oq := sqlmock.NewRows([]string{"organization_id", "resources"})
	if orgResources != "" {
		oq.AddRow(orgID, orgResources)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organization_quotas"`)).WillReturnRows(oq)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "organization_id", "resources"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "code_az", "resources"}))
}

func TestInitialize(t *testing.T) {
	orgID, projectID := uuid.New(), uuid.New()

	t.Run("applied: the project stays active", func(t *testing.T) {
		setupController(t, http.StatusNoContent)
		mock := setupMockDB(t)
		expectHierarchy(mock, orgID, `{"cpu":"8"}`)

		require.NoError(t, Initialize(context.Background(), orgID, projectID))
	})

	t.Run("not applied: the project is kept but deactivated", func(t *testing.T) {
		setupController(t, http.StatusInternalServerError)
		mock := setupMockDB(t)
		expectHierarchy(mock, orgID, `{"cpu":"8"}`)
		mock.ExpectBegin()
		mock.ExpectExec(sqlDeactivate).
			WithArgs(projectID, "quotaFailedToApply", "quota not applied on: az1", sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		err := Initialize(context.Background(), orgID, projectID)
		require.ErrorIs(t, err, ErrNotApplied)
	})

	t.Run("client gone: the quota is still applied", func(t *testing.T) {
		setupController(t, http.StatusNoContent)
		mock := setupMockDB(t)
		expectHierarchy(mock, orgID, `{"cpu":"8"}`)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.NoError(t, Initialize(ctx, orgID, projectID))
	})

	t.Run("nothing to enforce: the project stays active", func(t *testing.T) {
		setupController(t, http.StatusInternalServerError)
		mock := setupMockDB(t)
		expectHierarchy(mock, orgID, "")

		err := Initialize(context.Background(), orgID, projectID)
		require.ErrorIs(t, err, ErrNotApplied)
	})
}

func TestProjectReactivates(t *testing.T) {
	orgID, projectID := uuid.New(), uuid.New()

	t.Run("applied on every AZ: reactivated", func(t *testing.T) {
		setupController(t, http.StatusNoContent)
		mock := setupMockDB(t)
		expectHierarchy(mock, orgID, `{"cpu":"8"}`)
		mock.ExpectBegin()
		mock.ExpectExec(sqlReactivate).WithArgs(projectID, "quotaFailedToApply").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		require.NoError(t, Project(context.Background(), orgID, projectID))
	})

	t.Run("applied on some AZs only: not reactivated", func(t *testing.T) {
		setupController(t, http.StatusNoContent)
		mock := setupMockDB(t)
		expectHierarchy(mock, orgID, `{"cpu":"8"}`)

		require.NoError(t, Project(context.Background(), orgID, projectID, "az1"))
	})

	t.Run("not applied: not reactivated", func(t *testing.T) {
		setupController(t, http.StatusInternalServerError)
		mock := setupMockDB(t)
		expectHierarchy(mock, orgID, `{"cpu":"8"}`)

		require.ErrorIs(t, Project(context.Background(), orgID, projectID), ErrNotApplied)
	})
}
