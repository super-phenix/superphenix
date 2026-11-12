package quota

import (
	"errors"
	"fmt"
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
	r.Get("/organization/{orgaId}/quota", svc.GetOrganizationQuota)
	r.Get("/organization/{orgaId}/project/{projectId}/quota", svc.GetProjectQuota)
	r.Put("/organization/{orgaId}/project/{projectId}/quota", svc.SetProjectQuota)
	r.Put("/organization/{orgaId}/project/{projectId}/quota/az/{codeAZ}", svc.SetProjectAZQuota)
	r.Delete("/organization/{orgaId}/project/{projectId}/quota", svc.DeleteProjectQuota)
	return r
}

func do(r chi.Router, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func TestSetProjectQuotaRequestValidation(t *testing.T) {
	path := fmt.Sprintf("/organization/%s/project/%s/quota", uuid.New(), uuid.New())

	tests := map[string]struct {
		path   string
		body   string
		status int
	}{
		"invalid organization id": {path: "/organization/x/project/" + uuid.NewString() + "/quota", body: `{}`, status: http.StatusBadRequest},
		"invalid project id":      {path: "/organization/" + uuid.NewString() + "/project/x/quota", body: `{}`, status: http.StatusBadRequest},
		"malformed body":          {path: path, body: `{"cpu":`, status: http.StatusBadRequest},
		"non string quantity":     {path: path, body: `{"cpu":16}`, status: http.StatusBadRequest},
		"invalid quantity":        {path: path, body: `{"cpu":"lots"}`, status: http.StatusBadRequest},
		"translated key":          {path: path, body: `{"limits.cpu":"16"}`, status: http.StatusBadRequest},
		"invalid resource name":   {path: path, body: `{"pods: 1\nfoo":"16"}`, status: http.StatusBadRequest},
		"body too large":          {path: path, body: `{"cpu":"` + strings.Repeat("1", 2<<20) + `"}`, status: http.StatusRequestEntityTooLarge},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			setupMockDB(t) // no query is expected
			rr := do(newRouter(), http.MethodPut, tt.path, tt.body)
			assert.Equal(t, tt.status, rr.Code, rr.Body.String())
		})
	}
}

func TestSetProjectQuotaOfAnotherOrganization(t *testing.T) {
	orgID, projID := uuid.New(), uuid.New()
	mock := setupMockDB(t)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`FROM "organizations"`)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(orgID))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id" FROM "projects" WHERE (id = $1 AND orga_id = $2)`)).
		WithArgs(projID, orgID, 1).WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectRollback()

	rr := do(newRouter(), http.MethodPut, fmt.Sprintf("/organization/%s/project/%s/quota", orgID, projID), `{"cpu":"1"}`)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGetProjectQuotaOfAnotherOrganization(t *testing.T) {
	orgID, projID := uuid.New(), uuid.New()
	mock := setupMockDB(t)

	// No AZ controller is called for a project outside the organization.
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("AZ controller called: %s", r.URL.Path)
	}))
	defer controller.Close()
	old := config.Global.AZs
	config.Global.AZs = map[string]config.AZConfig{"az1": {Code: "az1", ControllerUrl: controller.URL}}
	t.Cleanup(func() { config.Global.AZs = old })

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id" FROM "projects" WHERE (id = $1 AND orga_id = $2)`)).
		WithArgs(projID, orgID, 1).WillReturnError(gorm.ErrRecordNotFound)

	rr := do(newRouter(), http.MethodGet, fmt.Sprintf("/organization/%s/project/%s/quota", orgID, projID), "")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestWriteError(t *testing.T) {
	tests := map[error]int{
		fmt.Errorf("%w: cpu", crudQuota.ErrInvalidQuantity):      http.StatusBadRequest,
		fmt.Errorf("%w: cpu", crudQuota.ErrOrgQuotaExceeded):     http.StatusBadRequest,
		fmt.Errorf("%w: cpu", crudQuota.ErrProjectQuotaExceeded): http.StatusBadRequest,
		crudQuota.ErrOrganizationNotFound:                        http.StatusNotFound,
		crudQuota.ErrProjectNotFound:                             http.StatusNotFound,
		crudQuota.ErrAZNotFound:                                  http.StatusNotFound,
		crudQuota.ErrProjectQuotaEnforced:                        http.StatusConflict,
	}
	for err, status := range tests {
		rr := httptest.NewRecorder()
		WriteError(rr, httptest.NewRequest(http.MethodGet, "/", nil), err, "generic")
		assert.Equal(t, status, rr.Code, err.Error())
		assert.Contains(t, rr.Body.String(), err.Error())
	}

	t.Run("internal errors are not leaked", func(t *testing.T) {
		rr := httptest.NewRecorder()
		WriteError(rr, httptest.NewRequest(http.MethodGet, "/", nil), errors.New(`pq: relation "secret_table" does not exist`), "generic")
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.NotContains(t, rr.Body.String(), "secret_table")
		assert.Contains(t, rr.Body.String(), "generic")
	})
}
