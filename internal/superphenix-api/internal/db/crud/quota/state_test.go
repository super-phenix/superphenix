package quota

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
)

var (
	sqlLockState    = regexp.QuoteMeta(`SELECT * FROM "project_az_quota_states" WHERE project_id = $1 AND code_az = $2 AND lease_token = $3 LIMIT $4 FOR UPDATE`)
	sqlDeleteOrgSt  = regexp.QuoteMeta(`DELETE FROM "project_az_quota_states" WHERE organization_id = $1`)
	sqlAllProjects  = regexp.QuoteMeta(`SELECT "id","orga_id" FROM "projects" WHERE "projects"."deleted_at" IS NULL ORDER BY orga_id,id`)
	sqlPrepareState = regexp.QuoteMeta(`UPDATE "project_az_quota_states" SET "held"=$1,"prepared_revision"=$2 ` +
		`WHERE project_id = $3 AND code_az = $4`)
	sqlPostponeState = regexp.QuoteMeta(`UPDATE "project_az_quota_states" SET "due_at"=now() + make_interval(secs => $1),` +
		`"last_error"=$2,"lease_token"=$3 WHERE project_id = $4 AND code_az = $5`)
	sqlRecordApplied = regexp.QuoteMeta(`UPDATE "project_az_quota_states" SET "applied_revision"=$1,"attempts"=$2,` +
		`"due_at"=CASE WHEN wanted_revision > $3 THEN now() ELSE now() + make_interval(secs => $4) END,` +
		`"held"=$5,"last_error"=$6,"lease_token"=$7 WHERE project_id = $8 AND code_az = $9 AND prepared_revision = $10`)
	sqlRecordFailed = regexp.QuoteMeta(`UPDATE "project_az_quota_states" SET "attempts"=attempts + 1,` +
		`"due_at"=CASE WHEN wanted_revision > $1 THEN now() ` +
		`ELSE now() + make_interval(secs => LEAST($2 * power(2, LEAST(attempts, 20)), $3)) END,` +
		`"last_error"=$4,"lease_token"=$5 WHERE project_id = $6 AND code_az = $7 AND prepared_revision = $8`)
)

// leaseQuery matches a claim query, due being its selection after WHERE.
func leaseQuery(due string) string {
	return regexp.QuoteMeta(`WITH due AS (SELECT "project_id","code_az" FROM "project_az_quota_states" WHERE ` + due +
		`) UPDATE project_az_quota_states AS s SET lease_token = `)
}

// keyRows returns the rows of the state keys returned by a claim.
func keyRows(keys ...StateKey) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"organization_id", "project_id", "code_az"})
	for _, key := range keys {
		rows.AddRow(key.OrganizationId, key.ProjectId, key.CodeAZ)
	}
	return rows
}

func TestClaim(t *testing.T) {
	token, key := uuid.New(), StateKey{OrganizationId: uuid.New(), ProjectId: uuid.New(), CodeAZ: "az-1"}

	t.Run("claims the states due in the given AZs", func(t *testing.T) {
		mock := setupMockDB(t)
		mock.ExpectQuery(leaseQuery(`due_at <= now() AND code_az IN ($1,$2) ORDER BY due_at LIMIT $3 FOR UPDATE SKIP LOCKED`)).
			WithArgs("az-1", "az-2", 10, token, time.Minute.Seconds()).WillReturnRows(keyRows(key))

		keys, err := Claim(context.Background(), token, []string{"az-1", "az-2"}, 10, time.Minute)
		require.NoError(t, err)
		assert.Equal(t, []StateKey{key}, keys)
	})

	t.Run("nothing to claim without AZ", func(t *testing.T) {
		setupMockDB(t)
		keys, err := Claim(context.Background(), token, nil, 10, time.Minute)
		require.NoError(t, err)
		assert.Empty(t, keys)
	})
}

func TestClaimDirty(t *testing.T) {
	token, organizationID, projectID := uuid.New(), uuid.New(), uuid.New()
	tests := []struct {
		name      string
		azs       []string
		projectID *uuid.UUID
		codeAZ    string
		// where and args are those of the claim, none when no state can be claimed.
		where string
		args  []driver.Value
	}{
		{"organization, in its AZs only", []string{"az-2", "az-1"}, nil, "",
			"organization_id = $1 AND code_az IN ($2,$3) AND wanted_revision > applied_revision",
			[]driver.Value{organizationID, "az-1", "az-2"}},
		{"project", []string{"az-1"}, &projectID, "",
			"(organization_id = $1 AND code_az IN ($2) AND wanted_revision > applied_revision) AND project_id = $3",
			[]driver.Value{organizationID, "az-1", projectID}},
		{"project in one AZ", []string{"az-1", "az-2"}, &projectID, "az-2",
			"(organization_id = $1 AND code_az IN ($2) AND wanted_revision > applied_revision) AND project_id = $3",
			[]driver.Value{organizationID, "az-2", projectID}},
		{"AZ the organization no longer has", []string{"az-1"}, &projectID, "az-old", "", nil},
		{"organization without AZ", nil, nil, "", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAZs(t, tt.azs...)
			mock := setupMockDB(t)
			key := StateKey{OrganizationId: organizationID, ProjectId: projectID, CodeAZ: "az-1"}
			var expected []StateKey
			if tt.where != "" {
				query := leaseQuery(tt.where + ` ORDER BY project_id, code_az COLLATE "C" FOR UPDATE`)
				args := append(append([]driver.Value{}, tt.args...), token, time.Minute.Seconds())
				mock.ExpectQuery(query).WithArgs(args...).WillReturnRows(keyRows(key))
				expected = []StateKey{key}
			}

			keys, err := ClaimDirty(context.Background(), token, organizationID, tt.projectID, tt.codeAZ, time.Minute)
			require.NoError(t, err)
			assert.Equal(t, expected, keys)
		})
	}
}

// expectOrganizationHierarchies expects the reads of readOrganizationHierarchies for the live
// projects projectIDs, project quotas being (project_id, resources), AZ quotas (project_id,
// code_az, resources).
func expectOrganizationHierarchies(mock sqlmock.Sqlmock, organizationID uuid.UUID, organizationResources string, projectIDs []uuid.UUID,
	projectQuotas [][2]any, azQuotas [][3]any, states ...stateRow) {
	expectOrgQuota(mock, organizationID, organizationResources)
	expectProjectIds(mock, organizationID, projectIDs...)
	expectListPQs(mock, organizationID, projectQuotas...)
	expectOrgAZQsOnly(mock, organizationID, azQuotas...)
	expectOrgStates(mock, organizationID, states...)
}

// expectLockState expects the state of key to be read for update, still claimed with token when
// state is not nil.
func expectLockState(mock sqlmock.Sqlmock, token uuid.UUID, key StateKey, state *stateRow) {
	query := mock.ExpectQuery(sqlLockState).WithArgs(key.ProjectId, key.CodeAZ, token, 1)
	if state == nil {
		query.WillReturnError(gorm.ErrRecordNotFound)
		return
	}
	query.WillReturnRows(stateRows(*state))
}

func TestPrepare(t *testing.T) {
	token, organizationID, projectID := uuid.New(), uuid.New(), uuid.New()
	const revision int64 = 1_700_000_000_000_000
	key := StateKey{OrganizationId: organizationID, ProjectId: projectID, CodeAZ: "az-1"}
	retryAfter := 10 * time.Minute

	expectPrepareStart := func(mock sqlmock.Sqlmock) {
		mock.ExpectBegin()
		expectLockOrg(mock, organizationID)
		mock.ExpectQuery(sqlBump).WithArgs(organizationID).WillReturnRows(sqlmock.NewRows([]string{"quota_revision"}).AddRow(revision))
	}

	t.Run("adds the AZ quota to the held quota and records the revision", func(t *testing.T) {
		setupAZs(t, "az-1")
		mock := setupMockDB(t)
		state := stateRow{projectID: projectID, codeAZ: "az-1", held: `{"cpu":"16","memory":"1Gi"}`, wantedRevision: 3, appliedRevision: 2}
		expectPrepareStart(mock)
		expectOrganizationHierarchies(mock, organizationID, `{"cpu":"64"}`, []uuid.UUID{projectID},
			[][2]any{{projectID, `{"cpu":"32"}`}}, [][3]any{{projectID, "az-1", `{"cpu":"8"}`}}, state)
		expectLockState(mock, token, key, &state)
		mock.ExpectExec(sqlPrepareState).
			WithArgs(`{"cpu":"16","memory":"1Gi"}`, revision, projectID, "az-1").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		pushes, err := Prepare(context.Background(), token, organizationID, []StateKey{key}, retryAfter)
		require.NoError(t, err)
		assert.Equal(t, []Push{{
			StateKey: key,
			Revision: revision,
			Hard:     map[string]string{"requests.cpu": "8", "limits.cpu": "8"},
			AZQuota:  model.QuotaResources{"cpu": "8"},
		}}, pushes)
	})

	t.Run("an AZ without AZ quota enforces 0 of what a parent limits", func(t *testing.T) {
		setupAZs(t, "az-1")
		mock := setupMockDB(t)
		state := stateRow{projectID: projectID, codeAZ: "az-1", wantedRevision: 1}
		expectPrepareStart(mock)
		expectOrganizationHierarchies(mock, organizationID, `{"cpu":"64"}`, []uuid.UUID{projectID}, nil, nil, state)
		expectLockState(mock, token, key, &state)
		mock.ExpectExec(sqlPrepareState).WithArgs(`{}`, revision, projectID, "az-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		pushes, err := Prepare(context.Background(), token, organizationID, []StateKey{key}, retryAfter)
		require.NoError(t, err)
		require.Len(t, pushes, 1)
		assert.Equal(t, map[string]string{"requests.cpu": "0", "limits.cpu": "0"}, pushes[0].Hard)
		assert.Nil(t, pushes[0].AZQuota)
	})

	t.Run("states are locked sorted by project then AZ", func(t *testing.T) {
		setupAZs(t, "az-1", "az-2")
		mock := setupMockDB(t)
		first, second := key, StateKey{OrganizationId: organizationID, ProjectId: projectID, CodeAZ: "az-2"}
		expectPrepareStart(mock)
		expectOrganizationHierarchies(mock, organizationID, "", []uuid.UUID{projectID}, nil, nil)
		expectLockState(mock, token, first, nil)
		expectLockState(mock, token, second, nil)
		mock.ExpectCommit()

		// Claimed by due date: az-2 first.
		_, err := Prepare(context.Background(), token, organizationID, []StateKey{second, first}, retryAfter)
		require.NoError(t, err)
	})

	t.Run("claimed since by another reconciler: skipped", func(t *testing.T) {
		setupAZs(t, "az-1")
		mock := setupMockDB(t)
		expectPrepareStart(mock)
		expectOrganizationHierarchies(mock, organizationID, "", []uuid.UUID{projectID}, nil, nil)
		expectLockState(mock, token, key, nil)
		mock.ExpectCommit()

		pushes, err := Prepare(context.Background(), token, organizationID, []StateKey{key}, retryAfter)
		require.NoError(t, err)
		assert.Empty(t, pushes)
	})

	t.Run("AZ no longer available to the organization: postponed with an error", func(t *testing.T) {
		setupAZs(t)
		mock := setupMockDB(t)
		state := stateRow{projectID: projectID, codeAZ: "az-1", wantedRevision: 1}
		expectPrepareStart(mock)
		expectOrganizationHierarchies(mock, organizationID, "", []uuid.UUID{projectID}, nil, nil, state)
		expectLockState(mock, token, key, &state)
		mock.ExpectExec(sqlPostponeState).
			WithArgs(retryAfter.Seconds(), "availability zone not available to the organization", nil, projectID, "az-1").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		pushes, err := Prepare(context.Background(), token, organizationID, []StateKey{key}, retryAfter)
		require.NoError(t, err)
		assert.Empty(t, pushes)
	})

	t.Run("deleted project: its states are deleted", func(t *testing.T) {
		setupAZs(t, "az-1")
		mock := setupMockDB(t)
		expectPrepareStart(mock)
		expectOrganizationHierarchies(mock, organizationID, "", nil, nil, nil)
		mock.ExpectExec(sqlDeleteProjectStates).WithArgs(projectID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		pushes, err := Prepare(context.Background(), token, organizationID, []StateKey{key}, retryAfter)
		require.NoError(t, err)
		assert.Empty(t, pushes)
	})

	t.Run("deleted organization: its states are deleted", func(t *testing.T) {
		mock := setupMockDB(t)
		mock.ExpectBegin()
		mock.ExpectQuery(sqlLockOrg).WithArgs(organizationID, 1).WillReturnError(gorm.ErrRecordNotFound)
		mock.ExpectExec(sqlDeleteOrgSt).WithArgs(organizationID).WillReturnResult(sqlmock.NewResult(0, 2))
		mock.ExpectCommit()

		pushes, err := Prepare(context.Background(), token, organizationID, []StateKey{key}, retryAfter)
		require.NoError(t, err)
		assert.Empty(t, pushes)
	})

	t.Run("a failure rolls back every state of the organization", func(t *testing.T) {
		setupAZs(t, "az-1")
		mock := setupMockDB(t)
		expectPrepareStart(mock)
		mock.ExpectQuery(sqlOrgQuota).WithArgs(organizationID, 1).WillReturnError(assert.AnError)
		mock.ExpectRollback()

		_, err := Prepare(context.Background(), token, organizationID, []StateKey{key}, retryAfter)
		require.ErrorIs(t, err, assert.AnError)
	})
}

func TestRecord(t *testing.T) {
	projectID := uuid.New()
	const revision int64 = 42
	push := Push{
		StateKey: StateKey{OrganizationId: uuid.New(), ProjectId: projectID, CodeAZ: "az-1"},
		Revision: revision,
		AZQuota:  model.QuotaResources{"cpu": "8"},
	}
	resync, backoff, maxBackoff := 10*time.Minute, 5*time.Second, 5*time.Minute
	tests := []struct {
		name    string
		push    Push
		pushErr error
		query   string
		args    []driver.Value
	}{
		{"applied: the AZ quota becomes the held quota", push, nil, sqlRecordApplied,
			[]driver.Value{revision, 0, revision, resync.Seconds(), `{"cpu":"8"}`, nil, nil, projectID, "az-1", revision}},
		{"applied without AZ quota: nothing held", Push{StateKey: push.StateKey, Revision: revision}, nil, sqlRecordApplied,
			[]driver.Value{revision, 0, revision, resync.Seconds(), `{}`, nil, nil, projectID, "az-1", revision}},
		{"failed: the error is recorded", push, errors.New("controller answered 500"), sqlRecordFailed,
			[]driver.Value{revision, backoff.Seconds(), maxBackoff.Seconds(), "controller answered 500", nil, projectID, "az-1", revision}},
		{"failed with a long error: truncated", push, errors.New(strings.Repeat("x", 2*maxErrorLength)), sqlRecordFailed,
			[]driver.Value{revision, backoff.Seconds(), maxBackoff.Seconds(), strings.Repeat("x", maxErrorLength), nil, projectID, "az-1", revision}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := setupMockDB(t)
			mock.ExpectExec(tt.query).WithArgs(tt.args...).WillReturnResult(sqlmock.NewResult(0, 1))

			require.NoError(t, Record(context.Background(), tt.push, tt.pushErr, resync, backoff, maxBackoff))
		})
	}
}

func TestEnsureStates(t *testing.T) {
	setupAZs(t, "az-1", "az-2")
	organizationID, firstProjectID, secondProjectID := uuid.New(), uuid.New(), uuid.New()
	mock := setupMockDB(t)

	mock.ExpectQuery(sqlAllProjects).WillReturnRows(sqlmock.NewRows([]string{"id", "orga_id"}).
		AddRow(firstProjectID, organizationID).AddRow(secondProjectID, organizationID))
	rows := sqlmock.NewRows([]string{"due_at"})
	for range 4 {
		rows.AddRow(time.Now())
	}
	// No transaction around the batches.
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "project_az_quota_states"`) + `.*` + regexp.QuoteMeta(`ON CONFLICT DO NOTHING`)).
		WillReturnRows(rows)

	require.NoError(t, EnsureStates(context.Background()))
}

func TestSortedKeys(t *testing.T) {
	first, second := uuid.MustParse("00000000-0000-0000-0000-000000000001"), uuid.MustParse("ff000000-0000-0000-0000-000000000000")
	keys := []StateKey{
		{ProjectId: second, CodeAZ: "az-1"},
		{ProjectId: first, CodeAZ: "az-b"},
		{ProjectId: first, CodeAZ: "AZ-c"},
		{ProjectId: first, CodeAZ: "az-a"},
	}
	// Byte-wise, as COLLATE "C": upper case first.
	assert.Equal(t, []StateKey{keys[2], keys[3], keys[1], keys[0]}, sortedKeys(keys))
	assert.Equal(t, second, keys[0].ProjectId, "the keys given are not reordered")
}

var sqlDeactivate = regexp.QuoteMeta(`INSERT INTO "project_deactivations"`) + `.*` +
	regexp.QuoteMeta(`ON CONFLICT ("project_id","reason") DO UPDATE SET "message"="excluded"."message","updated_at"="excluded"."updated_at"`)

func TestMarkNewProject(t *testing.T) {
	organizationID, projectID := uuid.New(), uuid.New()
	tests := []struct {
		name         string
		organization string
		markErr      error
		// expected are the AZs where the project is deactivated, none when it is not.
		expected []string
	}{
		{name: "nothing to enforce: marked, not deactivated"},
		{name: "something to enforce: deactivated and marked in one transaction", organization: `{"cpu":"8"}`,
			expected: []string{"az-1", "az-2"}},
		{name: "marking failed: nothing written", organization: `{"cpu":"8"}`, markErr: assert.AnError,
			expected: []string{"az-1", "az-2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAZs(t, "az-2", "az-1")
			mock := setupMockDB(t)
			expectProjectTransaction(mock, organizationID, projectID)
			expectHierarchy(mock, organizationID, projectID, tt.organization, "")
			if len(tt.expected) > 0 {
				mock.ExpectExec(sqlDeactivate).
					WithArgs(projectID, model.DeactivationQuotaFailedToApply, "quota not applied yet on: az-1, az-2", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			}
			if tt.markErr != nil {
				mock.ExpectQuery(sqlMarkDirty).WillReturnError(tt.markErr)
				mock.ExpectRollback()
			} else {
				expectMarkDirty(mock, 2)
				mock.ExpectCommit()
			}

			require.ErrorIs(t, MarkNewProject(context.Background(), organizationID, projectID), tt.markErr)
		})
	}
}

func TestUnappliedAZs(t *testing.T) {
	setupAZs(t, "az-1", "az-2", "az-3")
	organizationID, projectID := uuid.New(), uuid.New()
	mock := setupMockDB(t)

	// az-1 applied its quota, az-2 did not yet, az-3 has nothing to enforce.
	expectSnapshot(mock, organizationID, projectID, "", "", [][2]string{{"az-1", `{"cpu":"1"}`}, {"az-2", `{"cpu":"1"}`}}, []stateRow{
		{projectID: projectID, codeAZ: "az-1", wantedRevision: 2, appliedRevision: 2},
		{projectID: projectID, codeAZ: "az-2", wantedRevision: 2, appliedRevision: 1},
	})

	unapplied, err := UnappliedAZs(context.Background(), organizationID, projectID)
	require.NoError(t, err)
	assert.Equal(t, []string{"az-2"}, unapplied)
}

func TestStatus(t *testing.T) {
	failure := "controller answered 500"
	tests := []struct {
		name          string
		state         model.ProjectAZQuotaState
		expected      string
		expectedError string
	}{
		{"applied", model.ProjectAZQuotaState{WantedRevision: 2, AppliedRevision: 3}, StatusApplied, ""},
		{"pending", model.ProjectAZQuotaState{WantedRevision: 3, AppliedRevision: 2}, StatusPending, ""},
		{"never synced", model.ProjectAZQuotaState{}, StatusPending, ""},
		{"error, even once applied", model.ProjectAZQuotaState{WantedRevision: 2, AppliedRevision: 2, LastError: &failure}, StatusError, failure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, syncError := Status(tt.state)
			assert.Equal(t, tt.expected, status)
			assert.Equal(t, tt.expectedError, syncError)
		})
	}
}
