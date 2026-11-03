package quotasync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	crudQuota "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
)

var (
	sqlBump          = regexp.QuoteMeta(`UPDATE organizations SET quota_revision = GREATEST(quota_revision + 1,`)
	sqlLockOrg       = regexp.QuoteMeta(`SELECT "id" FROM "organizations" WHERE id = $1 AND "organizations"."deleted_at" IS NULL LIMIT $2 FOR UPDATE`)
	sqlCheckProject  = regexp.QuoteMeta(`SELECT "id" FROM "projects" WHERE (id = $1 AND orga_id = $2)`)
	sqlMarkDirty     = regexp.QuoteMeta(`INSERT INTO "project_az_quota_states"`)
	sqlClaimDirty    = regexp.QuoteMeta(`WITH due AS (SELECT project_id, code_az FROM project_az_quota_states WHERE organization_id = $1`)
	sqlDeactivate    = regexp.QuoteMeta(`INSERT INTO "project_deactivations"`) + `.*ON CONFLICT \("project_id","reason"\) DO UPDATE`
	sqlDeactivations = regexp.QuoteMeta(`SELECT * FROM "project_deactivations"`)
	sqlReactivate    = regexp.QuoteMeta(`DELETE FROM "project_deactivations" WHERE project_id = $1 AND reason = $2`)
)

func setupMockDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)

	oldClient := db.Client
	db.Client = gormDB
	oldWait := config.Global.Quota.ApplyWait
	config.Global.Quota.ApplyWait = time.Second
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		db.Client = oldClient
		config.Global.Quota.ApplyWait = oldWait
		_ = sqlDB.Close()
	})
	return mock
}

// setupController configures az1, whose controller answers status, and returns the bodies it was
// sent.
func setupController(t *testing.T, status int) *[]controllerBody {
	t.Helper()
	bodies := &[]controllerBody{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body controllerBody
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		*bodies = append(*bodies, body)
		w.WriteHeader(status)
		_, _ = w.Write([]byte("a newer quota revision is enforced: 12, pushed 10"))
	}))
	old := config.Global.AZs
	config.Global.AZs = map[string]config.AZConfig{"az1": {Code: "az1", ControllerUrl: srv.URL}}
	t.Cleanup(func() {
		srv.Close()
		config.Global.AZs = old
	})
	return bodies
}

// expectSnapshot expects the quotas of a project to be read in one read-only transaction, the
// organization limiting orgResources (none when empty), with the sync states of the project
// (code_az, applied: whether its last change is applied).
func expectSnapshot(mock sqlmock.Sqlmock, orgID uuid.UUID, orgResources string, states map[string]bool) {
	mock.ExpectBegin()
	oq := sqlmock.NewRows([]string{"organization_id", "resources"})
	if orgResources != "" {
		oq.AddRow(orgID, orgResources)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organization_quotas"`)).WillReturnRows(oq)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "organization_id", "resources"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "code_az", "resources"}))
	rows := sqlmock.NewRows([]string{"code_az", "wanted_revision", "applied_revision"})
	for code, applied := range states {
		appliedRevision := 1
		if applied {
			appliedRevision = 2
		}
		rows.AddRow(code, 2, appliedRevision)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quota_states"`)).WillReturnRows(rows)
	mock.ExpectCommit()
}

// expectMarkNewProject expects a project without quota to be marked dirty in az1 (see
// crudQuota.MarkNewProject). The marking fails with markErr when not nil.
func expectMarkNewProject(mock sqlmock.Sqlmock, orgID, projectID uuid.UUID, markErr error) {
	mock.ExpectBegin()
	mock.ExpectQuery(sqlLockOrg).WithArgs(orgID, 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(orgID))
	mock.ExpectQuery(sqlBump).WillReturnRows(sqlmock.NewRows([]string{"quota_revision"}).AddRow(41))
	mock.ExpectQuery(sqlCheckProject).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(projectID))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organization_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"organization_id", "resources"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "organization_id", "resources"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "code_az", "resources"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quota_states"`)).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "code_az"}))
	if markErr != nil {
		mock.ExpectQuery(sqlMarkDirty).WillReturnError(markErr)
		mock.ExpectRollback()
		return
	}
	mock.ExpectQuery(sqlMarkDirty).WillReturnRows(sqlmock.NewRows([]string{"due_at"}).AddRow(time.Now()))
	mock.ExpectCommit()
}

func TestPush(t *testing.T) {
	orgID, projectID := uuid.New(), uuid.New()
	tests := []struct {
		name          string
		status        int
		expectedError error
		message       string
	}{
		{"applied", http.StatusNoContent, nil, ""},
		{"below usage", http.StatusConflict, crudQuota.ErrBelowUsage, "in az1"},
		{"newer revision enforced", http.StatusPreconditionFailed, nil, "controller answered 412: a newer quota revision is enforced: 12, pushed 10"},
		{"controller failure", http.StatusInternalServerError, nil, "controller answered 500"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodies := setupController(t, tt.status)
			prepared := crudQuota.Push{
				StateKey: crudQuota.StateKey{OrganizationId: orgID, ProjectId: projectID, CodeAZ: "az1"},
				Revision: 10,
				Hard:     map[string]string{"requests.cpu": "8"},
			}

			err := pushToAZ(context.Background(), prepared)
			assert.Equal(t, []controllerBody{{Hard: prepared.Hard, Revision: 10}}, *bodies)
			switch {
			case tt.expectedError != nil:
				require.ErrorIs(t, err, tt.expectedError)
				assert.Contains(t, err.Error(), tt.message)
			case tt.message != "":
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.message)
			default:
				require.NoError(t, err)
			}
		})
	}

	t.Run("AZ not available to the organization", func(t *testing.T) {
		setupController(t, http.StatusNoContent)
		err := pushToAZ(context.Background(), crudQuota.Push{StateKey: crudQuota.StateKey{OrganizationId: orgID, CodeAZ: "az-unknown"}})
		require.Error(t, err)
	})
}

func TestApply(t *testing.T) {
	orgID, projectID := uuid.New(), uuid.New()
	scope := Scope{OrganizationId: orgID, ProjectId: &projectID}
	const claimDelay = 300 * time.Millisecond
	tests := []struct {
		name     string
		delay    time.Duration
		claimErr error
		expected bool
	}{
		{"nothing left to push", 0, nil, true},
		{"claim failed", 0, assert.AnError, false},
		{"still running after applyWait", claimDelay, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupController(t, http.StatusNoContent)
			mock := setupMockDB(t)
			config.Global.Quota.ApplyWait = claimDelay / 3
			claim := mock.ExpectQuery(sqlClaimDirty).WithArgs(orgID, "az1", projectID, sqlmock.AnyArg(), lease.Seconds()).WillDelayFor(tt.delay)
			if tt.claimErr != nil {
				claim.WillReturnError(tt.claimErr)
			} else {
				claim.WillReturnRows(sqlmock.NewRows([]string{"organization_id", "project_id", "code_az"}))
			}

			assert.Equal(t, tt.expected, Apply(context.Background(), scope))
			// Let a claim still running end before the mock is checked.
			time.Sleep(tt.delay)
		})
	}
}

func TestInitialize(t *testing.T) {
	orgID, projectID := uuid.New(), uuid.New()

	t.Run("applied", func(t *testing.T) {
		setupController(t, http.StatusNoContent)
		mock := setupMockDB(t)
		expectMarkNewProject(mock, orgID, projectID, nil)
		mock.ExpectQuery(sqlClaimDirty).WillReturnRows(sqlmock.NewRows([]string{"organization_id", "project_id", "code_az"}))

		require.NoError(t, Initialize(context.Background(), orgID, projectID))
	})

	t.Run("not applied in time", func(t *testing.T) {
		setupController(t, http.StatusNoContent)
		mock := setupMockDB(t)
		expectMarkNewProject(mock, orgID, projectID, nil)
		mock.ExpectQuery(sqlClaimDirty).WillReturnError(assert.AnError)

		require.ErrorIs(t, Initialize(context.Background(), orgID, projectID), ErrNotApplied)
	})

	t.Run("quota not computed: deactivated until the reconciler applies it", func(t *testing.T) {
		setupController(t, http.StatusNoContent)
		mock := setupMockDB(t)
		expectMarkNewProject(mock, orgID, projectID, assert.AnError)
		mock.ExpectBegin()
		mock.ExpectExec(sqlDeactivate).WithArgs(projectID, model.DeactivationQuotaFailedToApply, "quota not applied",
			sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		require.ErrorIs(t, Initialize(context.Background(), orgID, projectID), assert.AnError)
	})
}

func TestReactivate(t *testing.T) {
	orgID, projectID := uuid.New(), uuid.New()
	deactivations := func(reasons ...string) *sqlmock.Rows {
		rows := sqlmock.NewRows([]string{"project_id", "reason"})
		for _, reason := range reasons {
			rows.AddRow(projectID, reason)
		}
		return rows
	}

	t.Run("not deactivated for its quota", func(t *testing.T) {
		mock := setupMockDB(t)
		mock.ExpectQuery(sqlDeactivations).WillReturnRows(deactivations("other"))

		require.NoError(t, reactivate(context.Background(), orgID, projectID))
	})

	t.Run("reactivated once every AZ with something to enforce applied it", func(t *testing.T) {
		setupController(t, http.StatusNoContent)
		mock := setupMockDB(t)
		mock.ExpectQuery(sqlDeactivations).WillReturnRows(deactivations(model.DeactivationQuotaFailedToApply))
		expectSnapshot(mock, orgID, `{"cpu":"8"}`, map[string]bool{"az1": true})
		mock.ExpectBegin()
		mock.ExpectExec(sqlReactivate).WithArgs(projectID, model.DeactivationQuotaFailedToApply).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		require.NoError(t, reactivate(context.Background(), orgID, projectID))
	})

	t.Run("kept deactivated while an AZ did not apply it", func(t *testing.T) {
		setupController(t, http.StatusNoContent)
		mock := setupMockDB(t)
		mock.ExpectQuery(sqlDeactivations).WillReturnRows(deactivations(model.DeactivationQuotaFailedToApply))
		expectSnapshot(mock, orgID, `{"cpu":"8"}`, map[string]bool{"az1": false})

		require.NoError(t, reactivate(context.Background(), orgID, projectID))
	})
}
