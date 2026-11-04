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

	crudQuota "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	spxQuota "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/quota"
)

func TestGetProjectQuotaOverview(t *testing.T) {
	orgId, projectId := uuid.New(), uuid.New()
	controller := func(body string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/"+orgId.String()+"/"+projectId.String()+"/quota", r.URL.Path)
			assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(body))
		}))
	}
	az1 := controller(`{"hard":{"requests.cpu":"4","limits.cpu":"4"},"used":{"requests.cpu":"1","limits.cpu":"2500m","requests.memory":"1Gi"}}`)
	defer az1.Close()
	az2 := controller(`{"hard":{"requests.cpu":"4"},"used":{"requests.cpu":"500m","requests.storage":"10Gi"}}`)
	defer az2.Close()
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer down.Close()

	old := config.Global.AZs
	config.Global.AZs = map[string]config.AZConfig{
		"az1": {Code: "az1", ControllerUrl: az1.URL, AuthSecret: "secret"},
		"az2": {Code: "az2", ControllerUrl: az2.URL, AuthSecret: "secret"},
		"az3": {Code: "az3", ControllerUrl: down.URL, AuthSecret: "secret"},
	}
	t.Cleanup(func() { config.Global.AZs = old })

	mock := setupMockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id" FROM "projects"`)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(projectId))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organization_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"organization_id", "resources"}).AddRow(orgId, `{"cpu":"64","memory":"8Gi"}`))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "organization_id", "resources"}).AddRow(projectId, orgId, `{"cpu":"16"}`))
	// az1 has an AZ quota, az2 and az3 none; az-old is no longer available to the organization.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "code_az", "resources"}).
			AddRow(projectId, "az-old", `{"cpu":"1"}`).
			AddRow(projectId, "az1", `{"cpu":"4"}`))

	overview, err := GetProjectQuotaOverview(context.Background(), orgId, projectId)
	require.NoError(t, err)
	// memory is limited by the organization but not by the project: effectively 0.
	assert.Equal(t, model.QuotaResources{"cpu": "16", "memory": "0"}, overview.Limit)
	assert.Equal(t, model.QuotaResources{"cpu": "5", "memory": "0"}, overview.Allocated)
	assert.Equal(t, model.QuotaResources{"cpu": "11", "memory": "0"}, overview.Remaining)
	// Each AZ shows the limit it enforces: 0 for what a parent limits without AZ share.
	assert.Equal(t, []crudQuota.AZQuotaOverview{
		{CodeAZ: "az-old", Limit: model.QuotaResources{"cpu": "1", "memory": "0"}, Used: model.QuotaResources{}},
		{CodeAZ: "az1", Limit: model.QuotaResources{"cpu": "4", "memory": "0"}, Available: true, Used: model.QuotaResources{"cpu": "2500m", "memory": "1Gi"}},
		{CodeAZ: "az2", Limit: model.QuotaResources{"cpu": "0", "memory": "0"}, Available: true, Used: model.QuotaResources{"cpu": "500m", "disk": "10Gi"}},
		// An unreachable AZ is reported as unavailable.
		{CodeAZ: "az3", Limit: model.QuotaResources{"cpu": "0", "memory": "0"}, Used: model.QuotaResources{}},
	}, overview.AZQuotas)

	// The reader registered for the crud checks gives the enforced and used quantities of one AZ.
	azUsage, err := crudQuota.ReadUsage(context.Background(), orgId, projectId, "az1")
	require.NoError(t, err)
	assert.Equal(t, spxQuota.Quantities{"cpu": 4000}, azUsage.Hard)
	assert.Equal(t, spxQuota.Quantities{"cpu": 2500, "memory": 1 << 30}, azUsage.Used)
	_, err = crudQuota.ReadUsage(context.Background(), orgId, projectId, "az3")
	require.Error(t, err)
	_, err = crudQuota.ReadUsage(context.Background(), orgId, projectId, "az-unknown")
	require.Error(t, err)
}
