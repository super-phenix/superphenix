package quota

import (
	"bytes"
	"encoding/json"
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
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
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

func TestGetOrganizationQuota(t *testing.T) {
	orgID := uuid.New()

	t.Run("success returns quota and project allocations", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organization_quotas" WHERE organization_id = $1 AND "organization_quotas"."deleted_at" IS NULL ORDER BY "organization_quotas"."organization_id" LIMIT $2`)).
			WithArgs(orgID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"organization_id", "resources"}).
				AddRow(orgID, `{"cpu":"64","memory":"256Gi","disk":"2Ti"}`))

		proj1 := uuid.New()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas" WHERE organization_id = $1 AND "project_quotas"."deleted_at" IS NULL`)).
			WithArgs(orgID).
			WillReturnRows(sqlmock.NewRows([]string{"project_id", "organization_id", "resources"}).
				AddRow(proj1, orgID, `{"cpu":"32","memory":"128Gi","disk":"1Ti"}`))

		r := chi.NewRouter()
		svc := New(&config.Global)
		r.Get("/organization/{orgaId}/quota", svc.GetOrganizationQuota)

		req := httptest.NewRequest(http.MethodGet, "/organization/"+orgID.String()+"/quota", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var resp OrganizationQuotaOverview
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.False(t, resp.Unlimited)
		assert.Equal(t, "64", resp.Resources.CPU)
		assert.Equal(t, "32", resp.Allocated.CPU)
		assert.Equal(t, "32", resp.Remaining.CPU)
		assert.Len(t, resp.ProjectQuotas, 1)
	})
}

func TestGetProjectQuota(t *testing.T) {
	orgID := uuid.New()
	projID := uuid.New()

	mock, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas" WHERE project_id = $1 AND "project_quotas"."deleted_at" IS NULL ORDER BY "project_quotas"."project_id" LIMIT $2`)).
		WithArgs(projID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "organization_id", "resources"}).
			AddRow(projID, orgID, `{"cpu":"32","memory":"128Gi","disk":"1Ti"}`))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas" WHERE project_id = $1 AND "project_az_quotas"."deleted_at" IS NULL`)).
		WithArgs(projID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "code_az", "resources"}).
			AddRow(uuid.New(), projID, "az-1", `{"cpu":"16","memory":"64Gi","disk":"500Gi"}`))

	r := chi.NewRouter()
	svc := New(&config.Global)
	r.Get("/organization/{orgaId}/project/{projectId}/quota", svc.GetProjectQuota)

	req := httptest.NewRequest(http.MethodGet, "/organization/"+orgID.String()+"/project/"+projID.String()+"/quota", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp ProjectQuotaOverview
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "32", resp.Resources.CPU)
	assert.Len(t, resp.AZQuotas, 1)
}

func TestSetProjectQuota(t *testing.T) {
	orgID := uuid.New()
	projID := uuid.New()

	config.Global.AZs = map[string]config.AZConfig{
		"az-1": {Code: "az-1"},
	}

	mock, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organization_quotas" WHERE organization_id = $1 AND "organization_quotas"."deleted_at" IS NULL ORDER BY "organization_quotas"."organization_id" LIMIT $2`)).
		WithArgs(orgID, 1).
		WillReturnError(gorm.ErrRecordNotFound) // unlimited org

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas" WHERE project_id = $1 AND "project_quotas"."deleted_at" IS NULL ORDER BY "project_quotas"."project_id" LIMIT $2`)).
		WithArgs(projID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "project_quotas"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas" WHERE (project_id = $1 AND code_az = $2) AND "project_az_quotas"."deleted_at" IS NULL`)).
		WithArgs(projID, "az-1", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "project_az_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	mock.ExpectCommit()

	r := chi.NewRouter()
	svc := New(&config.Global)
	r.Put("/organization/{orgaId}/project/{projectId}/quota", svc.SetProjectQuota)

	body, _ := json.Marshal(model.QuotaResources{
		CPU:    "16",
		Memory: "64Gi",
		Disk:   "1Ti",
	})
	req := httptest.NewRequest(http.MethodPut, "/organization/"+orgID.String()+"/project/"+projID.String()+"/quota", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDeleteProjectQuota(t *testing.T) {
	orgID := uuid.New()
	projID := uuid.New()

	mock, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "project_quotas" SET "deleted_at"=$1 WHERE project_id = $2 AND "project_quotas"."deleted_at" IS NULL`)).
		WithArgs(sqlmock.AnyArg(), projID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	r := chi.NewRouter()
	svc := New(&config.Global)
	r.Delete("/organization/{orgaId}/project/{projectId}/quota", svc.DeleteProjectQuota)

	req := httptest.NewRequest(http.MethodDelete, "/organization/"+orgID.String()+"/project/"+projID.String()+"/quota", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}
