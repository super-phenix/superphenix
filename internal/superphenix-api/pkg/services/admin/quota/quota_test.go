package quota

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	crudQuota "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
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

func newRouter() chi.Router {
	svc := New(&config.Global)
	r := chi.NewRouter()
	r.Get("/organization/{orgaId}", svc.GetOrganizationQuota)
	r.Put("/organization/{orgaId}", svc.SetOrganizationQuota)
	r.Delete("/organization/{orgaId}", svc.DeleteOrganizationQuota)
	return r
}

func TestAdminGetOrganizationQuota(t *testing.T) {
	orgID, projectID := uuid.New(), uuid.New()
	mock := setupMockDB(t)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organization_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"organization_id", "resources"}).AddRow(orgID, `{"cpu":"64"}`))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "organization_id", "resources"}).AddRow(projectID, orgID, `{"cpu":"16"}`))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "code_az", "resources"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id" FROM "projects" WHERE orga_id = $1`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(projectID))

	rr := httptest.NewRecorder()
	newRouter().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/organization/"+orgID.String(), nil))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp crudQuota.OrganizationQuotaOverview
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.False(t, resp.Unlimited)
	assert.Equal(t, "16", resp.Allocated["cpu"])
	assert.Equal(t, "48", resp.Remaining["cpu"])
}

func TestAdminSetOrganizationQuota(t *testing.T) {
	orgID := uuid.New()

	t.Run("unknown organization", func(t *testing.T) {
		mock := setupMockDB(t)
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`FROM "organizations"`)).WillReturnError(gorm.ErrRecordNotFound)
		mock.ExpectRollback()

		rr := httptest.NewRecorder()
		newRouter().ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/organization/"+orgID.String(), strings.NewReader(`{"cpu":"64"}`)))
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("invalid resources", func(t *testing.T) {
		setupMockDB(t)
		rr := httptest.NewRecorder()
		newRouter().ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/organization/"+orgID.String(), strings.NewReader(`{"requests.memory":"1Gi"}`)))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

// fakeController records the quotas pushed to an AZ controller, answering status.
func fakeController(t *testing.T, status int) map[string]map[string]string {
	t.Helper()
	pushed := map[string]map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Hard map[string]string `json:"hard"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		pushed[r.Method+" "+r.URL.Path] = body.Hard
		w.WriteHeader(status)
	}))
	old := config.Global.AZs
	config.Global.AZs = map[string]config.AZConfig{"az1": {Code: "az1", ControllerUrl: srv.URL}}
	t.Cleanup(func() {
		srv.Close()
		config.Global.AZs = old
	})
	return pushed
}

func expectDeleteAndSync(mock sqlmock.Sqlmock, orgID, projectID uuid.UUID) {
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "organization_quotas" WHERE organization_id = $1`)).
		WithArgs(orgID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id" FROM "projects" WHERE orga_id = $1`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(projectID))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organization_quotas"`)).WillReturnRows(sqlmock.NewRows([]string{"organization_id"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "organization_id", "resources"}).AddRow(projectID, orgID, `{"cpu":"4","disk":"10Gi"}`))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "code_az", "resources"}).AddRow(projectID, "az1", `{"cpu":"2"}`))
}

func TestAdminDeleteOrganizationQuota(t *testing.T) {
	orgID, projectID := uuid.New(), uuid.New()

	t.Run("pushes the remaining project quota to the AZ controllers", func(t *testing.T) {
		pushed := fakeController(t, http.StatusNoContent)
		mock := setupMockDB(t)
		expectDeleteAndSync(mock, orgID, projectID)
		// Applied on every AZ: a project deactivated by a failed quota push is reactivated.
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "project_deactivations" WHERE project_id = $1 AND reason = $2`)).
			WithArgs(projectID, "quotaFailedToApply").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		rr := httptest.NewRecorder()
		newRouter().ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/organization/"+orgID.String(), nil))
		assert.Equal(t, http.StatusNoContent, rr.Code)
		// The project still limits disk, without AZ share: enforced at 0.
		assert.Equal(t, map[string]string{"requests.cpu": "2", "limits.cpu": "2", "requests.storage": "0"},
			pushed["PUT /"+orgID.String()+"/"+projectID.String()+"/quota"])
	})

	t.Run("controller failure", func(t *testing.T) {
		fakeController(t, http.StatusInternalServerError)
		expectDeleteAndSync(setupMockDB(t), orgID, projectID)

		rr := httptest.NewRecorder()
		newRouter().ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/organization/"+orgID.String(), nil))
		assert.Equal(t, http.StatusBadGateway, rr.Code)
	})
}
