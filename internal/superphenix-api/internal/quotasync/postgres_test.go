package quotasync

// Tests of the reconciler against a real Postgres, for what sqlmock cannot check: row locks,
// SKIP LOCKED claims and the fencing of the outcomes. They are skipped unless QUOTA_TEST_DSN
// holds a Postgres DSN (e.g. "host=localhost port=5433 user=superphenix password=secret
// dbname=superphenix"); they run in a schema of their own, dropped afterwards.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	crudQuota "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/quota"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
)

// setupPostgres points db.Client to a new schema of the QUOTA_TEST_DSN database, with the tables
// of a new database (see model.AutoMigrate).
func setupPostgres(t *testing.T) {
	t.Helper()
	dsn := os.Getenv("QUOTA_TEST_DSN")
	if dsn == "" {
		t.Skip("QUOTA_TEST_DSN not set")
	}
	schema := "quota_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		assert.NoError(t, admin.Exec("DROP SCHEMA "+schema+" CASCADE").Error)
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	client, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	previousClient := db.Client
	db.Client = client
	t.Cleanup(func() {
		db.Client = previousClient
		if sqlDB, err := client.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, model.AutoMigrate(client))
}

// fakeAZ is an AZ controller keeping the quota of each project in memory, refusing a push older
// than the revision it holds (412), like the superphenix-controller.
type fakeAZ struct {
	mutex  sync.Mutex
	quotas map[string]controllerBody
	pushes int
}

// setupFakeAZs configures the AZs codes, all served by one fakeAZ.
func setupFakeAZs(t *testing.T, codes ...string) *fakeAZ {
	t.Helper()
	fake := &fakeAZ{quotas: map[string]controllerBody{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The quota of a project in an AZ is keyed by the AZ (first path segment) and the project.
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		key := parts[0] + "/" + parts[2]
		fake.mutex.Lock()
		defer fake.mutex.Unlock()
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(controllerStatus{Used: map[string]string{}})
			return
		}
		var body controllerBody
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		fake.pushes++
		if fake.quotas[key].Revision > body.Revision {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		fake.quotas[key] = body
		w.WriteHeader(http.StatusNoContent)
	}))
	previousAZs := config.Global.AZs
	config.Global.AZs = map[string]config.AZConfig{}
	for _, code := range codes {
		config.Global.AZs[code] = config.AZConfig{Code: code, ControllerUrl: srv.URL + "/" + code}
	}
	t.Cleanup(func() {
		srv.Close()
		config.Global.AZs = previousAZs
	})
	return fake
}

// enforced returns the quota the fake AZ code enforces for the project.
func (fake *fakeAZ) enforced(code string, projectId uuid.UUID) controllerBody {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return fake.quotas[code+"/"+projectId.String()]
}

// createProjects creates an organization with count projects.
func createProjects(t *testing.T, count int) (uuid.UUID, []uuid.UUID) {
	t.Helper()
	owner := model.User{Email: "owner@example.com"}
	require.NoError(t, db.Client.Create(&owner).Error)
	organization := model.Organization{Name: "quota test", OwnerId: owner.ID}
	require.NoError(t, db.Client.Create(&organization).Error)
	projectIds := make([]uuid.UUID, count)
	for index := range projectIds {
		project := model.Project{Name: fmt.Sprintf("project %d", index), OrgaId: organization.ID}
		require.NoError(t, db.Client.Create(&project).Error)
		projectIds[index] = project.ID
	}
	return organization.ID, projectIds
}

// state reads the sync state of a project in an AZ.
func state(t *testing.T, projectId uuid.UUID, code string) model.ProjectAZQuotaState {
	t.Helper()
	var state model.ProjectAZQuotaState
	require.NoError(t, db.Client.Where("project_id = ? AND code_az = ?", projectId, code).Take(&state).Error)
	return state
}

// expireLeases makes every state due, as when the lease of a reconciler elapsed.
func expireLeases(t *testing.T) {
	t.Helper()
	require.NoError(t, db.Client.Exec("UPDATE project_az_quota_states SET due_at = now()").Error)
}

// claimAndPrepare claims the states due and prepares their pushes, as a reconciler does before
// pushing.
func claimAndPrepare(t *testing.T, organizationId uuid.UUID) []crudQuota.Push {
	t.Helper()
	token := uuid.New()
	keys, err := crudQuota.Claim(context.Background(), token, configuredAZCodes(), claimBatch, lease)
	require.NoError(t, err)
	pushes, err := crudQuota.Prepare(context.Background(), token, organizationId, keys, time.Hour)
	require.NoError(t, err)
	return pushes
}

// Two reconcilers pushing at once never claim the same quota, and every quota converges.
func TestPostgresReconcilersConverge(t *testing.T) {
	setupPostgres(t)
	fake := setupFakeAZs(t, "az1")
	organizationId, projectIds := createProjects(t, 20)
	_, err := crudQuota.SetOrganizationQuota(context.Background(), organizationId, model.QuotaResources{"cpu": "40"})
	require.NoError(t, err)

	var group sync.WaitGroup
	for range 2 {
		group.Go(func() {
			NewReconciler(config.QuotaResyncConfig{Interval: time.Hour}).ReconcileDue(context.Background())
		})
	}
	group.Wait()

	assert.Equal(t, len(projectIds), fake.pushes, "each quota is pushed once")
	for _, projectId := range projectIds {
		assert.Equal(t, map[string]string{"requests.cpu": "2", "limits.cpu": "2"}, fake.enforced("az1", projectId).Hard)
		unapplied, err := crudQuota.UnappliedAZs(context.Background(), organizationId, projectId)
		require.NoError(t, err)
		assert.Empty(t, unapplied)
	}
}

// A reconciler whose lease elapsed records nothing once another prepared a newer push, whether its
// push reached the AZ first (overwritten by the newer one) or last (refused by the AZ).
func TestPostgresDeposedReconcilerIsFenced(t *testing.T) {
	for _, deposedPushesFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("deposed push first: %v", deposedPushesFirst), func(t *testing.T) {
			setupPostgres(t)
			fake := setupFakeAZs(t, "az1")
			organizationId, projectIds := createProjects(t, 1)
			projectId := projectIds[0]
			_, err := crudQuota.SetProjectQuota(context.Background(), organizationId, projectId, model.QuotaResources{"cpu": "8"})
			require.NoError(t, err)

			deposed := claimAndPrepare(t, organizationId)
			require.Len(t, deposed, 1)
			expireLeases(t)
			current := claimAndPrepare(t, organizationId)
			require.Len(t, current, 1)
			require.Greater(t, current[0].Revision, deposed[0].Revision)

			record := func(push crudQuota.Push) {
				require.NoError(t, crudQuota.Record(context.Background(), push, pushToAZ(context.Background(), push), time.Hour, time.Second, time.Minute))
			}
			if deposedPushesFirst {
				deposedErr := pushToAZ(context.Background(), deposed[0])
				require.NoError(t, deposedErr)
				record(current[0])
				require.NoError(t, crudQuota.Record(context.Background(), deposed[0], deposedErr, time.Hour, time.Second, time.Minute))
			} else {
				record(current[0])
				record(deposed[0])
			}

			assert.Equal(t, current[0].Revision, fake.enforced("az1", projectId).Revision)
			synced := state(t, projectId, "az1")
			assert.Equal(t, current[0].Revision, synced.AppliedRevision)
			assert.Nil(t, synced.LastError)
			assert.Nil(t, synced.LeaseToken)
		})
	}
}

// A lowered AZ quota frees nothing until the AZ applies it, even when a push prepared before the
// lowering lands after it.
func TestPostgresLoweringDuringPush(t *testing.T) {
	setupPostgres(t)
	fake := setupFakeAZs(t, "az1", "az2")
	organizationId, projectIds := createProjects(t, 1)
	projectId := projectIds[0]
	ctx := context.Background()
	_, err := crudQuota.SetProjectQuota(ctx, organizationId, projectId, model.QuotaResources{"cpu": "16"})
	require.NoError(t, err)
	NewReconciler(config.QuotaResyncConfig{Interval: time.Hour}).ReconcileDue(ctx)
	for _, code := range []string{"az1", "az2"} {
		require.Equal(t, map[string]string{"requests.cpu": "8", "limits.cpu": "8"}, fake.enforced(code, projectId).Hard)
		require.Equal(t, crudQuota.StatusApplied, statusOf(state(t, projectId, code)))
	}

	// A periodic push of az1 (8 cpu) is prepared, then az1 is lowered to 2 before it is sent.
	expireLeases(t)
	inFlight := claimAndPrepare(t, organizationId)
	_, err = crudQuota.SetProjectAZQuota(ctx, organizationId, projectId, "az1", model.QuotaResources{"cpu": "2"})
	require.NoError(t, err)

	// az1 may still enforce 8: az2 cannot take the 6 freed yet.
	_, err = crudQuota.SetProjectAZQuota(ctx, organizationId, projectId, "az2", model.QuotaResources{"cpu": "14"})
	require.ErrorIs(t, err, crudQuota.ErrProjectQuotaExceeded)

	// The old push lands and is recorded: the change made since is still to be pushed.
	for _, push := range inFlight {
		require.NoError(t, crudQuota.Record(ctx, push, pushToAZ(ctx, push), time.Hour, time.Second, time.Minute))
	}
	assert.Equal(t, crudQuota.StatusPending, statusOf(state(t, projectId, "az1")))

	NewReconciler(config.QuotaResyncConfig{Interval: time.Hour}).ReconcileDue(ctx)
	assert.Equal(t, map[string]string{"requests.cpu": "2", "limits.cpu": "2"}, fake.enforced("az1", projectId).Hard)
	assert.Equal(t, crudQuota.StatusApplied, statusOf(state(t, projectId, "az1")))
	_, err = crudQuota.SetProjectAZQuota(ctx, organizationId, projectId, "az2", model.QuotaResources{"cpu": "14"})
	require.NoError(t, err)
}

// A quota claimed by a reconciler that crashed is pushed by another one once its lease elapses.
func TestPostgresCrashAfterPrepare(t *testing.T) {
	setupPostgres(t)
	fake := setupFakeAZs(t, "az1")
	organizationId, projectIds := createProjects(t, 1)
	ctx := context.Background()
	_, err := crudQuota.SetProjectQuota(ctx, organizationId, projectIds[0], model.QuotaResources{"cpu": "4"})
	require.NoError(t, err)

	// Claimed and prepared, then nothing: the reconciler crashed.
	require.Len(t, claimAndPrepare(t, organizationId), 1)
	reconciler := NewReconciler(config.QuotaResyncConfig{Interval: time.Hour})
	reconciler.ReconcileDue(ctx)
	assert.Zero(t, fake.pushes, "leased to the crashed reconciler")

	expireLeases(t)
	reconciler.ReconcileDue(ctx)
	assert.Equal(t, 1, fake.pushes)
	assert.Equal(t, crudQuota.StatusApplied, statusOf(state(t, projectIds[0], "az1")))
}

func statusOf(state model.ProjectAZQuotaState) string {
	status, _ := crudQuota.Status(state)
	return status
}

// A dirty state of an AZ the organization no longer has does not keep the writes of its project
// from being applied.
func TestPostgresClaimDirtyIgnoresRemovedAZ(t *testing.T) {
	setupPostgres(t)
	setupFakeAZs(t, "az1", "az2")
	previousWait := config.Global.Quota.ApplyWait
	config.Global.Quota.ApplyWait = 5 * time.Second
	t.Cleanup(func() { config.Global.Quota.ApplyWait = previousWait })
	organizationId, projectIds := createProjects(t, 1)
	projectId := projectIds[0]
	ctx := context.Background()
	_, err := crudQuota.SetProjectQuota(ctx, organizationId, projectId, model.QuotaResources{"cpu": "8"})
	require.NoError(t, err)

	// az2 is removed before its quota is applied.
	delete(config.Global.AZs, "az2")
	assert.True(t, Apply(ctx, Scope{OrganizationId: organizationId, ProjectId: &projectId}))
	assert.Equal(t, crudQuota.StatusApplied, statusOf(state(t, projectId, "az1")))
	assert.Equal(t, crudQuota.StatusPending, statusOf(state(t, projectId, "az2")), "left to the reconciler")
}

// Prepare locks the states in the order ClaimDirty locks them (ORDER BY project_id, code_az
// COLLATE "C"), whatever the order of the keys it is given, so that neither waits for the other
// while holding a lock the other waits for. Prepare returns its pushes in the order it locked them.
func TestPostgresLockOrder(t *testing.T) {
	setupPostgres(t)
	setupFakeAZs(t, "az-b", "AZ-c", "az-a")
	organizationId, projectIds := createProjects(t, 4)
	ctx := context.Background()
	for _, projectId := range projectIds {
		_, err := crudQuota.SetProjectQuota(ctx, organizationId, projectId, model.QuotaResources{"cpu": "3"})
		require.NoError(t, err)
	}

	var locked []crudQuota.StateKey
	require.NoError(t, db.Client.Raw(`SELECT organization_id, project_id, code_az FROM project_az_quota_states `+
		`ORDER BY project_id, code_az COLLATE "C"`).Scan(&locked).Error)
	require.Len(t, locked, 12)

	token := uuid.New()
	keys, err := crudQuota.ClaimDirty(ctx, token, organizationId, nil, "", lease)
	require.NoError(t, err)
	slices.Reverse(keys)
	pushes, err := crudQuota.Prepare(ctx, token, organizationId, keys, time.Hour)
	require.NoError(t, err)
	prepared := make([]crudQuota.StateKey, len(pushes))
	for index, push := range pushes {
		prepared[index] = push.StateKey
	}
	assert.Equal(t, locked, prepared)
}
