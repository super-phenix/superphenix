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

func TestAdminGetOrganizationQuota(t *testing.T) {
	orgID := uuid.New()

	t.Run("success returns org quota and project allocations", func(t *testing.T) {
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
		r.Get("/quota/organization/{orgaId}", svc.GetOrganizationQuota)

		req := httptest.NewRequest(http.MethodGet, "/quota/organization/"+orgID.String(), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var resp AdminOrganizationQuotaResponse
		err := json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.False(t, resp.Unlimited)
		assert.Equal(t, "64", resp.Resources.CPU)
		assert.Equal(t, "32", resp.Allocated.CPU)
		assert.Equal(t, "32", resp.Remaining.CPU)
		assert.Len(t, resp.ProjectQuotas, 1)
	})
}

func TestAdminSetOrganizationQuota(t *testing.T) {
	orgID := uuid.New()

	mock, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "projects" WHERE orga_id = $1 AND "projects"."deleted_at" IS NULL`)).
		WithArgs(orgID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "orga_id", "name"}))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas" WHERE organization_id = $1 AND "project_quotas"."deleted_at" IS NULL`)).
		WithArgs(orgID).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "organization_id", "resources"}))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organization_quotas" WHERE organization_id = $1 AND "organization_quotas"."deleted_at" IS NULL ORDER BY "organization_quotas"."organization_id" LIMIT $2`)).
		WithArgs(orgID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "organization_quotas"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	r := chi.NewRouter()
	svc := New(&config.Global)
	r.Put("/quota/organization/{orgaId}", svc.SetOrganizationQuota)

	body, _ := json.Marshal(model.QuotaResources{
		CPU:    "64",
		Memory: "256Gi",
		Disk:   "2Ti",
	})
	req := httptest.NewRequest(http.MethodPut, "/quota/organization/"+orgID.String(), bytes.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAdminDeleteOrganizationQuota(t *testing.T) {
	orgID := uuid.New()

	mock, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "organization_quotas" SET "deleted_at"=$1 WHERE organization_id = $2 AND "organization_quotas"."deleted_at" IS NULL`)).
		WithArgs(sqlmock.AnyArg(), orgID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	r := chi.NewRouter()
	svc := New(&config.Global)
	r.Delete("/quota/organization/{orgaId}", svc.DeleteOrganizationQuota)

	req := httptest.NewRequest(http.MethodDelete, "/quota/organization/"+orgID.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}
