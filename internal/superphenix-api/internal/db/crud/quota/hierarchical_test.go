package quota

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
)

// The SQL queries of hierarchical.go, as matched by sqlmock (regular expressions, matching a
// prefix of the query).
var (
	sqlLockOrg      = regexp.QuoteMeta(`SELECT "id" FROM "organizations" WHERE id = $1 AND "organizations"."deleted_at" IS NULL LIMIT $2 FOR UPDATE`)
	sqlCheckProject = regexp.QuoteMeta(`SELECT "id" FROM "projects" WHERE (id = $1 AND orga_id = $2) AND "projects"."deleted_at" IS NULL LIMIT $3`)
	sqlOrgQuota     = regexp.QuoteMeta(`SELECT * FROM "organization_quotas" WHERE organization_id = $1 AND "organization_quotas"."deleted_at" IS NULL LIMIT $2`)
	sqlProjectQuota = regexp.QuoteMeta(`SELECT * FROM "project_quotas" WHERE project_id = $1 AND "project_quotas"."deleted_at" IS NULL LIMIT $2`)
	sqlListPQs      = regexp.QuoteMeta(`SELECT * FROM "project_quotas" WHERE (organization_id = $1 AND project_id IN (SELECT "id" FROM "projects" WHERE orga_id = $2 AND "projects"."deleted_at" IS NULL)) AND "project_quotas"."deleted_at" IS NULL ORDER BY project_id`)
	sqlProjectAZs   = regexp.QuoteMeta(`SELECT * FROM "project_az_quotas" WHERE project_id = $1 AND "project_az_quotas"."deleted_at" IS NULL ORDER BY code_az`)
	sqlOrgAZQs      = regexp.QuoteMeta(`SELECT * FROM "project_az_quotas" WHERE project_id IN (SELECT "id" FROM "projects" WHERE orga_id = $1 AND "projects"."deleted_at" IS NULL) AND "project_az_quotas"."deleted_at" IS NULL ORDER BY project_id, code_az`)
	sqlProjectIds   = regexp.QuoteMeta(`SELECT "id" FROM "projects" WHERE orga_id = $1 AND "projects"."deleted_at" IS NULL ORDER BY id`)
	sqlUpsertOQ     = regexp.QuoteMeta(`INSERT INTO "organization_quotas"`) + `.*ON CONFLICT \("organization_id"\) DO UPDATE SET "resources"="excluded"."resources"`
	sqlUpsertPQ     = regexp.QuoteMeta(`INSERT INTO "project_quotas"`) + `.*ON CONFLICT \("project_id"\) DO UPDATE SET "organization_id"="excluded"."organization_id","resources"="excluded"."resources"`
	sqlUpsertAZQ    = regexp.QuoteMeta(`INSERT INTO "project_az_quotas"`) + `.*ON CONFLICT \("project_id","code_az"\) DO UPDATE SET "resources"="excluded"."resources"`
)

// ---------------------------------------------------------------------------------------------
// Test setup
// ---------------------------------------------------------------------------------------------

// setupMockDB replaces db.Client by a sqlmock for the test, and checks at the end of the test
// that every expected query was run.
func setupMockDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	sqlDatabase, mock, err := sqlmock.New()
	require.NoError(t, err)

	gormDatabase, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDatabase}), &gorm.Config{})
	require.NoError(t, err)

	previousClient := db.Client
	db.Client = gormDatabase
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		db.Client = previousClient
		_ = sqlDatabase.Close()
	})
	return mock
}

// setupAZs configures the AZs available to every organization for the test.
func setupAZs(t *testing.T, codes ...string) {
	t.Helper()
	previousAZs := config.Global.AZs
	config.Global.AZs = map[string]config.AZConfig{}
	for _, code := range codes {
		config.Global.AZs[code] = config.AZConfig{Code: code}
	}
	t.Cleanup(func() { config.Global.AZs = previousAZs })
}

// setupUsage makes ReadUsage answer usage (or err), recording the AZs it is called for.
func setupUsage(t *testing.T, usage AZUsage, err error) *[]string {
	t.Helper()
	calledAZs := &[]string{}
	previousReader := ReadUsage
	ReadUsage = func(_ context.Context, _, _ uuid.UUID, codeAZ string) (AZUsage, error) {
		*calledAZs = append(*calledAZs, codeAZ)
		return usage, err
	}
	t.Cleanup(func() { ReadUsage = previousReader })
	return calledAZs
}

// setupPush makes PushAZ answer err, recording the quotas it pushes.
func setupPush(t *testing.T, err error) *[]map[string]string {
	t.Helper()
	pushedQuotas := &[]map[string]string{}
	previousPusher := PushAZ
	PushAZ = func(_ context.Context, _, _ uuid.UUID, _ string, hard map[string]string) error {
		*pushedQuotas = append(*pushedQuotas, hard)
		return err
	}
	t.Cleanup(func() { PushAZ = previousPusher })
	return pushedQuotas
}

// ---------------------------------------------------------------------------------------------
// Expected queries. A resources argument "" means that the quota does not exist.
// ---------------------------------------------------------------------------------------------

// expectOrganizationTransaction expects a transaction to start and lock the organization (see
// inOrganizationTransaction).
func expectOrganizationTransaction(mock sqlmock.Sqlmock, organizationID uuid.UUID) {
	mock.ExpectBegin()
	expectLockOrg(mock, organizationID)
}

// expectProjectTransaction expects a transaction to start, lock the organization and check that
// the project belongs to it (see inProjectTransaction).
func expectProjectTransaction(mock sqlmock.Sqlmock, organizationID, projectID uuid.UUID) {
	expectOrganizationTransaction(mock, organizationID)
	expectProject(mock, organizationID, projectID)
}

func expectLockOrg(mock sqlmock.Sqlmock, organizationID uuid.UUID) {
	mock.ExpectQuery(sqlLockOrg).WithArgs(organizationID, 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(organizationID))
}

func expectProject(mock sqlmock.Sqlmock, organizationID, projectID uuid.UUID) {
	mock.ExpectQuery(sqlCheckProject).WithArgs(projectID, organizationID, 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(projectID))
}

func expectOrgQuota(mock sqlmock.Sqlmock, organizationID uuid.UUID, resources string) {
	query := mock.ExpectQuery(sqlOrgQuota).WithArgs(organizationID, 1)
	if resources == "" {
		query.WillReturnError(gorm.ErrRecordNotFound)
		return
	}
	query.WillReturnRows(sqlmock.NewRows([]string{"organization_id", "resources"}).AddRow(organizationID, resources))
}

func expectProjectQuota(mock sqlmock.Sqlmock, organizationID, projectID uuid.UUID, resources string) {
	query := mock.ExpectQuery(sqlProjectQuota).WithArgs(projectID, 1)
	if resources == "" {
		query.WillReturnError(gorm.ErrRecordNotFound)
		return
	}
	query.WillReturnRows(sqlmock.NewRows([]string{"project_id", "organization_id", "resources"}).AddRow(projectID, organizationID, resources))
}

// expectProjectAZQs expects the AZ quotas of a project, rows being (code_az, resources).
func expectProjectAZQs(mock sqlmock.Sqlmock, projectID uuid.UUID, rows ...[2]string) {
	result := sqlmock.NewRows([]string{"project_id", "code_az", "resources"})
	for _, row := range rows {
		result.AddRow(projectID, row[0], row[1])
	}
	mock.ExpectQuery(sqlProjectAZs).WithArgs(projectID).WillReturnRows(result)
}

// expectHierarchy expects the reads of loadHierarchy: the organization quota, the project quota
// and the AZ quotas of the project, azQuotas being (code_az, resources).
func expectHierarchy(mock sqlmock.Sqlmock, organizationID, projectID uuid.UUID, organizationResources, projectResources string, azQuotas ...[2]string) {
	expectOrgQuota(mock, organizationID, organizationResources)
	expectProjectQuota(mock, organizationID, projectID, projectResources)
	expectProjectAZQs(mock, projectID, azQuotas...)
}

// expectListPQs expects the project quotas of the live projects of the organization, rows being
// (project_id, resources).
func expectListPQs(mock sqlmock.Sqlmock, organizationID uuid.UUID, rows ...[2]any) {
	result := sqlmock.NewRows([]string{"project_id", "organization_id", "resources"})
	for _, row := range rows {
		result.AddRow(row[0], organizationID, row[1])
	}
	mock.ExpectQuery(sqlListPQs).WithArgs(organizationID, organizationID).WillReturnRows(result)
}

// expectOrgAZQs expects the AZ quotas of the live projects of the organization, rows being
// (project_id, code_az, resources).
func expectOrgAZQs(mock sqlmock.Sqlmock, organizationID uuid.UUID, rows ...[3]any) {
	result := sqlmock.NewRows([]string{"project_id", "code_az", "resources"})
	for _, row := range rows {
		result.AddRow(row[0], row[1], row[2])
	}
	mock.ExpectQuery(sqlOrgAZQs).WithArgs(organizationID).WillReturnRows(result)
}

// expectProjectIds expects the IDs of the live projects of the organization (see listProjectIds).
func expectProjectIds(mock sqlmock.Sqlmock, organizationID uuid.UUID, projectIDs ...uuid.UUID) {
	result := sqlmock.NewRows([]string{"id"})
	for _, projectID := range projectIDs {
		result.AddRow(projectID)
	}
	mock.ExpectQuery(sqlProjectIds).WithArgs(organizationID).WillReturnRows(result)
}

func expectUpsertOrganizationQuota(mock sqlmock.Sqlmock) {
	mock.ExpectExec(sqlUpsertOQ).WillReturnResult(sqlmock.NewResult(1, 1))
}

func expectUpsertPQ(mock sqlmock.Sqlmock, organizationID, projectID uuid.UUID, resources string) {
	mock.ExpectExec(sqlUpsertPQ).
		WithArgs(projectID, organizationID, sqlmock.AnyArg(), sqlmock.AnyArg(), nil, resources).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func expectUpsertAZQ(mock sqlmock.Sqlmock, projectID uuid.UUID, codeAZ, resources string) {
	mock.ExpectQuery(sqlUpsertAZQ).
		WithArgs(projectID, codeAZ, sqlmock.AnyArg(), sqlmock.AnyArg(), nil, resources).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
}

// ---------------------------------------------------------------------------------------------
// GetOrganizationQuotaOverview
// ---------------------------------------------------------------------------------------------

func TestGetOrganizationQuotaOverview(t *testing.T) {
	organizationID := uuid.New()
	firstProjectID, secondProjectID, thirdProjectID := uuid.New(), uuid.New(), uuid.New()

	t.Run("limited", func(t *testing.T) {
		mock := setupMockDB(t)
		expectOrgQuota(mock, organizationID, `{"cpu":"64","memory":"256Gi","disk":"100Gi"}`)
		expectListPQs(mock, organizationID,
			[2]any{firstProjectID, `{"cpu":"16","memory":"64Gi","disk":"50Gi"}`},
			[2]any{secondProjectID, `{"cpu":"500m"}`})
		// The AZ quotas of a project with a project quota are not counted twice, those of
		// a project without quota are its allocation.
		expectOrgAZQs(mock, organizationID,
			[3]any{firstProjectID, "az-1", `{"cpu":"8"}`},
			[3]any{thirdProjectID, "az-1", `{"cpu":"1"}`},
			[3]any{thirdProjectID, "az-2", `{"cpu":"1","memory":"1Gi"}`})
		expectProjectIds(mock, organizationID, firstProjectID, secondProjectID, thirdProjectID)

		overview, err := GetOrganizationQuotaOverview(context.Background(), organizationID)
		require.NoError(t, err)
		assert.False(t, overview.Unlimited)
		assert.Equal(t, model.QuotaResources{"cpu": "18500m", "memory": "65Gi", "disk": "50Gi"}, overview.Allocated)
		assert.Equal(t, model.QuotaResources{"cpu": "45500m", "memory": "191Gi", "disk": "50Gi"}, overview.Remaining)
		// Every project is listed with its effective limit: 0 for what the organization limits
		// and the project quota does not (or when there is no project quota).
		assert.Equal(t, []model.ProjectQuota{
			{ProjectId: firstProjectID, OrganizationId: organizationID, Resources: model.QuotaResources{"cpu": "16", "memory": "64Gi", "disk": "50Gi"}},
			{ProjectId: secondProjectID, OrganizationId: organizationID, Resources: model.QuotaResources{"cpu": "500m", "memory": "0", "disk": "0"}},
			{ProjectId: thirdProjectID, OrganizationId: organizationID, Resources: model.QuotaResources{"cpu": "0", "memory": "0", "disk": "0"}},
		}, overview.ProjectQuotas)
	})

	t.Run("allocated is 0 for a limited resource nobody holds", func(t *testing.T) {
		mock := setupMockDB(t)
		expectOrgQuota(mock, organizationID, `{"cpu":"8","memory":"1Gi"}`)
		expectListPQs(mock, organizationID)
		expectOrgAZQs(mock, organizationID)
		expectProjectIds(mock, organizationID)

		overview, err := GetOrganizationQuotaOverview(context.Background(), organizationID)
		require.NoError(t, err)
		assert.Equal(t, model.QuotaResources{"cpu": "0", "memory": "0"}, overview.Allocated)
		assert.Equal(t, model.QuotaResources{"cpu": "8", "memory": "1Gi"}, overview.Remaining)
		assert.Empty(t, overview.ProjectQuotas)
	})

	t.Run("unlimited", func(t *testing.T) {
		mock := setupMockDB(t)
		expectOrgQuota(mock, organizationID, "")
		expectListPQs(mock, organizationID)
		expectOrgAZQs(mock, organizationID)

		overview, err := GetOrganizationQuotaOverview(context.Background(), organizationID)
		require.NoError(t, err)
		assert.True(t, overview.Unlimited)
		assert.Nil(t, overview.Remaining)
	})
}

// ---------------------------------------------------------------------------------------------
// SetOrganizationQuota
// ---------------------------------------------------------------------------------------------

func TestSetOrganizationQuota(t *testing.T) {
	organizationID, firstProjectID, secondProjectID := uuid.New(), uuid.New(), uuid.New()

	t.Run("auto-split among projects and AZs when no project quota exists", func(t *testing.T) {
		setupAZs(t, "az-2", "az-1")
		mock := setupMockDB(t)

		expectOrganizationTransaction(mock, organizationID)
		expectListPQs(mock, organizationID)
		expectOrgAZQs(mock, organizationID)
		expectProjectIds(mock, organizationID, firstProjectID, secondProjectID)
		for _, projectID := range []uuid.UUID{firstProjectID, secondProjectID} {
			expectUpsertPQ(mock, organizationID, projectID, `{"cpu":"32","disk":"1"}`)
			// A share that rounds down to 0 stays limited at 0.
			expectUpsertAZQ(mock, projectID, "az-1", `{"cpu":"16","disk":"0"}`)
			expectUpsertAZQ(mock, projectID, "az-2", `{"cpu":"16","disk":"0"}`)
		}
		expectUpsertOrganizationQuota(mock)
		mock.ExpectCommit()

		organizationQuota, err := SetOrganizationQuota(context.Background(), organizationID, model.QuotaResources{"cpu": "64", "disk": "2"})
		require.NoError(t, err)
		assert.Equal(t, "64", organizationQuota.Resources["cpu"])
	})

	t.Run("auto-split keeps what the AZ quotas already hold", func(t *testing.T) {
		setupAZs(t, "az-1")
		mock := setupMockDB(t)

		expectOrganizationTransaction(mock, organizationID)
		expectListPQs(mock, organizationID)
		// The first project already has 8 cpu in its AZ quotas (set while the organization had
		// no quota).
		expectOrgAZQs(mock, organizationID, [3]any{firstProjectID, "az-1", `{"cpu":"8","memory":"1Gi"}`})
		expectProjectIds(mock, organizationID, firstProjectID, secondProjectID)
		// The 12 cpu left are split equally; memory, not limited by the organization, is not
		// added. The AZ quotas of the first project are kept as they are.
		expectUpsertPQ(mock, organizationID, firstProjectID, `{"cpu":"14"}`)
		expectUpsertPQ(mock, organizationID, secondProjectID, `{"cpu":"6"}`)
		expectUpsertAZQ(mock, secondProjectID, "az-1", `{"cpu":"6"}`)
		expectUpsertOrganizationQuota(mock)
		mock.ExpectCommit()

		_, err := SetOrganizationQuota(context.Background(), organizationID, model.QuotaResources{"cpu": "20"})
		require.NoError(t, err)
	})

	t.Run("cannot be set below what the projects are allocated", func(t *testing.T) {
		// 32 + 33 = 65 cpu allocated: exactly 65 or more is accepted, below is refused,
		// whether it is the first organization quota or a lowered one.
		for cpu, accepted := range map[string]bool{"65": true, "100": true, "64": false, "30": false} {
			mock := setupMockDB(t)

			expectOrganizationTransaction(mock, organizationID)
			expectListPQs(mock, organizationID, [2]any{firstProjectID, `{"cpu":"32"}`}, [2]any{secondProjectID, `{"cpu":"33"}`})
			expectOrgAZQs(mock, organizationID)
			if accepted {
				expectUpsertOrganizationQuota(mock)
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}

			_, err := SetOrganizationQuota(context.Background(), organizationID, model.QuotaResources{"cpu": cpu})
			if accepted {
				require.NoError(t, err, cpu)
			} else {
				require.ErrorIs(t, err, ErrBelowAllocated, cpu)
			}
		}
	})

	t.Run("AZ quotas above their project quota count", func(t *testing.T) {
		mock := setupMockDB(t)

		expectOrganizationTransaction(mock, organizationID)
		expectListPQs(mock, organizationID, [2]any{firstProjectID, `{"cpu":"2"}`})
		expectOrgAZQs(mock, organizationID, [3]any{firstProjectID, "az-1", `{"cpu":"5"}`}, [3]any{firstProjectID, "az-2", `{"cpu":"5"}`})
		mock.ExpectRollback()

		_, err := SetOrganizationQuota(context.Background(), organizationID, model.QuotaResources{"cpu": "8"})
		require.ErrorIs(t, err, ErrBelowAllocated)
	})

	t.Run("a resource the projects do not hold can be limited", func(t *testing.T) {
		mock := setupMockDB(t)

		expectOrganizationTransaction(mock, organizationID)
		expectListPQs(mock, organizationID, [2]any{firstProjectID, `{"cpu":"32"}`})
		expectOrgAZQs(mock, organizationID)
		expectUpsertOrganizationQuota(mock)
		mock.ExpectCommit()

		_, err := SetOrganizationQuota(context.Background(), organizationID, model.QuotaResources{"cpu": "32", "memory": "1Gi"})
		require.NoError(t, err)
	})

	t.Run("project quotas get what their AZ quotas hold of a newly limited resource", func(t *testing.T) {
		mock := setupMockDB(t)

		expectOrganizationTransaction(mock, organizationID)
		// The first project does not limit memory, the second has no project quota: both hold
		// some in their AZs.
		expectListPQs(mock, organizationID, [2]any{firstProjectID, `{"cpu":"4"}`})
		expectOrgAZQs(mock, organizationID,
			[3]any{firstProjectID, "az-1", `{"cpu":"4","memory":"2Gi"}`},
			[3]any{secondProjectID, "az-1", `{"memory":"1Gi","disk":"1Gi"}`})
		// The project quotas are written in the order of the project IDs.
		upserts := []struct {
			projectID uuid.UUID
			resources string
		}{{firstProjectID, `{"cpu":"4","memory":"2Gi"}`}, {secondProjectID, `{"memory":"1Gi"}`}}
		if secondProjectID.String() < firstProjectID.String() {
			upserts[0], upserts[1] = upserts[1], upserts[0]
		}
		for _, upsert := range upserts {
			expectUpsertPQ(mock, organizationID, upsert.projectID, upsert.resources)
		}
		expectUpsertOrganizationQuota(mock)
		mock.ExpectCommit()

		_, err := SetOrganizationQuota(context.Background(), organizationID, model.QuotaResources{"cpu": "8", "memory": "4Gi"})
		require.NoError(t, err)
	})

	t.Run("unknown organization", func(t *testing.T) {
		mock := setupMockDB(t)

		mock.ExpectBegin()
		mock.ExpectQuery(sqlLockOrg).WithArgs(organizationID, 1).WillReturnError(gorm.ErrRecordNotFound)
		mock.ExpectRollback()

		_, err := SetOrganizationQuota(context.Background(), organizationID, model.QuotaResources{"cpu": "64"})
		require.ErrorIs(t, err, ErrOrganizationNotFound)
	})

	t.Run("invalid resources are rejected before any query", func(t *testing.T) {
		setupMockDB(t)
		_, err := SetOrganizationQuota(context.Background(), organizationID, model.QuotaResources{"pods": "64"})
		require.ErrorIs(t, err, ErrInvalidQuantity)
	})
}

// ---------------------------------------------------------------------------------------------
// SetProjectQuota
// ---------------------------------------------------------------------------------------------

func TestSetProjectQuota(t *testing.T) {
	organizationID, projectID, otherProjectID := uuid.New(), uuid.New(), uuid.New()

	t.Run("within the organization quota, split among AZs", func(t *testing.T) {
		setupAZs(t, "az-1")
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectHierarchy(mock, organizationID, projectID, `{"cpu":"64","memory":"256Gi"}`, `{"cpu":"60"}`)
		expectListPQs(mock, organizationID, [2]any{otherProjectID, `{"cpu":"32","memory":"128Gi"}`}, [2]any{projectID, `{"cpu":"60"}`})
		expectOrgAZQs(mock, organizationID)
		expectUpsertPQ(mock, organizationID, projectID, `{"cpu":"32","memory":"128Gi"}`)
		expectUpsertAZQ(mock, projectID, "az-1", `{"cpu":"32","memory":"128Gi"}`)
		mock.ExpectCommit()

		projectQuota, err := SetProjectQuota(context.Background(), organizationID, projectID, model.QuotaResources{"cpu": "32", "memory": "128Gi"})
		require.NoError(t, err)
		assert.Equal(t, organizationID, projectQuota.OrganizationId)
	})

	t.Run("AZ quotas already defined are kept", func(t *testing.T) {
		setupAZs(t, "az-1", "az-2")
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectHierarchy(mock, organizationID, projectID, "", `{"cpu":"8"}`, [2]string{"az-1", `{"cpu":"4"}`}, [2]string{"az-2", `{"cpu":"4"}`})
		expectUpsertPQ(mock, organizationID, projectID, `{"cpu":"8"}`)
		// No AZ quota upsert: the project admin's distribution is not overwritten.
		mock.ExpectCommit()

		_, err := SetProjectQuota(context.Background(), organizationID, projectID, model.QuotaResources{"cpu": "8"})
		require.NoError(t, err)
	})

	t.Run("cannot be set below its AZ quotas", func(t *testing.T) {
		// Even without any organization quota, and on a resource limited for the first time.
		for _, requested := range []model.QuotaResources{{"cpu": "8"}, {"cpu": "9", "memory": "512Mi"}} {
			mock := setupMockDB(t)
			expectProjectTransaction(mock, organizationID, projectID)
			expectHierarchy(mock, organizationID, projectID, "", "",
				[2]string{"az-1", `{"cpu":"4"}`}, [2]string{"az-2", `{"cpu":"5","memory":"1Gi"}`})
			mock.ExpectRollback()

			_, err := SetProjectQuota(context.Background(), organizationID, projectID, requested)
			require.ErrorIs(t, err, ErrBelowAllocated)
			assert.Contains(t, err.Error(), "its availability zones")
		}
	})

	t.Run("unlimited organization", func(t *testing.T) {
		setupAZs(t)
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectHierarchy(mock, organizationID, projectID, "", "")
		expectUpsertPQ(mock, organizationID, projectID, `{"disk":"1Ti"}`)
		mock.ExpectCommit()

		_, err := SetProjectQuota(context.Background(), organizationID, projectID, model.QuotaResources{"disk": "1Ti"})
		require.NoError(t, err)
	})

	t.Run("exceeds the organization quota", func(t *testing.T) {
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectHierarchy(mock, organizationID, projectID, `{"cpu":"64"}`, "")
		expectListPQs(mock, organizationID, [2]any{otherProjectID, `{"cpu":"48"}`})
		expectOrgAZQs(mock, organizationID)
		mock.ExpectRollback()

		_, err := SetProjectQuota(context.Background(), organizationID, projectID, model.QuotaResources{"cpu": "17"})
		require.ErrorIs(t, err, ErrOrgQuotaExceeded)
	})

	t.Run("omitting a resource limited by the organization counts as 0", func(t *testing.T) {
		setupAZs(t)
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectHierarchy(mock, organizationID, projectID, `{"cpu":"64","memory":"1Gi"}`, "")
		expectListPQs(mock, organizationID, [2]any{otherProjectID, `{"cpu":"32","memory":"1Gi"}`})
		expectOrgAZQs(mock, organizationID)
		expectUpsertPQ(mock, organizationID, projectID, `{"cpu":"32"}`)
		mock.ExpectCommit()

		_, err := SetProjectQuota(context.Background(), organizationID, projectID, model.QuotaResources{"cpu": "32"})
		require.NoError(t, err)
	})

	t.Run("cannot remove a resource the organization limits while its AZ quotas hold some", func(t *testing.T) {
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectHierarchy(mock, organizationID, projectID, `{"cpu":"64","memory":"64Gi"}`, `{"cpu":"8","memory":"1Gi"}`,
			[2]string{"az-1", `{"cpu":"4","memory":"1Gi"}`})
		mock.ExpectRollback()

		// memory is no longer set: it would count as 0, below the 1Gi of az-1.
		_, err := SetProjectQuota(context.Background(), organizationID, projectID, model.QuotaResources{"cpu": "8"})
		require.ErrorIs(t, err, ErrBelowAllocated)
		assert.Contains(t, err.Error(), "memory")
	})

	t.Run("resource not limited by the organization", func(t *testing.T) {
		setupAZs(t)
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectHierarchy(mock, organizationID, projectID, `{"cpu":"64"}`, "")
		expectListPQs(mock, organizationID)
		expectOrgAZQs(mock, organizationID)
		expectUpsertPQ(mock, organizationID, projectID, `{"cpu":"1","memory":"1Ti"}`)
		mock.ExpectCommit()

		_, err := SetProjectQuota(context.Background(), organizationID, projectID, model.QuotaResources{"cpu": "1", "memory": "1Ti"})
		require.NoError(t, err)
	})

	t.Run("project quota above the organization (stored before) can be lowered", func(t *testing.T) {
		setupAZs(t)
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectHierarchy(mock, organizationID, projectID, `{"cpu":"64","memory":"64Gi"}`, `{"cpu":"40","memory":"8Gi"}`)
		expectListPQs(mock, organizationID, [2]any{otherProjectID, `{"cpu":"48","memory":"48Gi"}`}, [2]any{projectID, `{"cpu":"40","memory":"8Gi"}`})
		expectOrgAZQs(mock, organizationID)
		expectUpsertPQ(mock, organizationID, projectID, `{"cpu":"30","memory":"8Gi"}`)
		mock.ExpectCommit()

		// 48 + 30 > 64 cpu, still above the organization, but lowered.
		_, err := SetProjectQuota(context.Background(), organizationID, projectID, model.QuotaResources{"cpu": "30", "memory": "8Gi"})
		require.NoError(t, err)
	})

	t.Run("project quota above the organization (stored before) cannot be increased", func(t *testing.T) {
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectHierarchy(mock, organizationID, projectID, `{"cpu":"64","memory":"64Gi"}`, `{"cpu":"40","memory":"8Gi"}`)
		expectListPQs(mock, organizationID, [2]any{otherProjectID, `{"cpu":"48","memory":"48Gi"}`}, [2]any{projectID, `{"cpu":"40","memory":"8Gi"}`})
		expectOrgAZQs(mock, organizationID)
		mock.ExpectRollback()

		// cpu is lowered, but memory goes beyond what is left (48 + 20 > 64).
		_, err := SetProjectQuota(context.Background(), organizationID, projectID, model.QuotaResources{"cpu": "30", "memory": "20Gi"})
		require.ErrorIs(t, err, ErrOrgQuotaExceeded)
		assert.Contains(t, err.Error(), "memory")
	})

	t.Run("a resource overcommitted by other projects does not block other resources", func(t *testing.T) {
		setupAZs(t)
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectHierarchy(mock, organizationID, projectID, `{"cpu":"64","memory":"64Gi"}`, `{"cpu":"8"}`)
		expectListPQs(mock, organizationID, [2]any{otherProjectID, `{"cpu":"16","memory":"80Gi"}`}, [2]any{projectID, `{"cpu":"8"}`})
		expectOrgAZQs(mock, organizationID)
		expectUpsertPQ(mock, organizationID, projectID, `{"cpu":"32"}`)
		mock.ExpectCommit()

		_, err := SetProjectQuota(context.Background(), organizationID, projectID, model.QuotaResources{"cpu": "32"})
		require.NoError(t, err)
	})

	t.Run("AZ quotas of a project without quota count against the organization quota", func(t *testing.T) {
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectHierarchy(mock, organizationID, projectID, `{"cpu":"64"}`, "")
		expectListPQs(mock, organizationID)
		// Set before the organization had a quota: 24 + 16 = 40 cpu.
		expectOrgAZQs(mock, organizationID,
			[3]any{otherProjectID, "az-1", `{"cpu":"24"}`},
			[3]any{otherProjectID, "az-2", `{"cpu":"16"}`})
		mock.ExpectRollback()

		_, err := SetProjectQuota(context.Background(), organizationID, projectID, model.QuotaResources{"cpu": "25"})
		require.ErrorIs(t, err, ErrOrgQuotaExceeded)
	})

	t.Run("lowering a project quota below its AZ quotas frees nothing for other projects", func(t *testing.T) {
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectHierarchy(mock, organizationID, projectID, `{"cpu":"10"}`, "")
		// The other project was lowered to 2 cpu, but its AZ quotas still enforce 10.
		expectListPQs(mock, organizationID, [2]any{otherProjectID, `{"cpu":"2"}`})
		expectOrgAZQs(mock, organizationID,
			[3]any{otherProjectID, "az-1", `{"cpu":"5"}`},
			[3]any{otherProjectID, "az-2", `{"cpu":"5"}`})
		mock.ExpectRollback()

		_, err := SetProjectQuota(context.Background(), organizationID, projectID, model.QuotaResources{"cpu": "8"})
		require.ErrorIs(t, err, ErrOrgQuotaExceeded)
	})

	t.Run("project of another organization", func(t *testing.T) {
		mock := setupMockDB(t)

		expectOrganizationTransaction(mock, organizationID)
		mock.ExpectQuery(sqlCheckProject).WithArgs(projectID, organizationID, 1).WillReturnError(gorm.ErrRecordNotFound)
		mock.ExpectRollback()

		_, err := SetProjectQuota(context.Background(), organizationID, projectID, model.QuotaResources{"cpu": "1"})
		require.ErrorIs(t, err, ErrProjectNotFound)
	})
}

// ---------------------------------------------------------------------------------------------
// SetProjectAZQuota
// ---------------------------------------------------------------------------------------------

func TestSetProjectAZQuota(t *testing.T) {
	organizationID, projectID := uuid.New(), uuid.New()

	// expectAZHierarchy expects the reads of loadHierarchy for a project with an AZ quota in both
	// az-1 and az-2.
	expectAZHierarchy := func(mock sqlmock.Sqlmock, organizationResources, projectResources, firstAZResources, secondAZResources string) {
		expectHierarchy(mock, organizationID, projectID, organizationResources, projectResources,
			[2]string{"az-1", firstAZResources}, [2]string{"az-2", secondAZResources})
	}

	t.Run("within the project quota", func(t *testing.T) {
		setupAZs(t, "az-1", "az-2")
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectAZHierarchy(mock, "", `{"cpu":"32","disk":"20Gi"}`, `{"cpu":"1"}`, `{"cpu":"16","disk":"10Gi"}`)
		expectUpsertAZQ(mock, projectID, "az-1", `{"cpu":"16","disk":"10Gi"}`)
		mock.ExpectCommit()

		azQuota, err := SetProjectAZQuota(context.Background(), organizationID, projectID, "az-1", model.QuotaResources{"cpu": "16", "disk": "10Gi"})
		require.NoError(t, err)
		assert.Equal(t, "az-1", azQuota.CodeAZ)
	})

	t.Run("exceeds the project quota", func(t *testing.T) {
		setupAZs(t, "az-1", "az-2")
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectAZHierarchy(mock, "", `{"cpu":"32"}`, `{"cpu":"1"}`, `{"cpu":"20"}`)
		mock.ExpectRollback()

		_, err := SetProjectAZQuota(context.Background(), organizationID, projectID, "az-1", model.QuotaResources{"cpu": "16"})
		require.ErrorIs(t, err, ErrProjectQuotaExceeded)
	})

	t.Run("AZ quota above the project (stored before) can be lowered", func(t *testing.T) {
		setupAZs(t, "az-1", "az-2")
		calledAZs := setupUsage(t, AZUsage{Hard: map[string]int64{"cpu": 30000}, Used: map[string]int64{"cpu": 25000}}, nil)
		t.Cleanup(func() { assert.Equal(t, []string{"az-1"}, *calledAZs) })
		pushedQuotas := setupPush(t, nil)
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectAZHierarchy(mock, "", `{"cpu":"32"}`, `{"cpu":"30"}`, `{"cpu":"20"}`)
		expectUpsertAZQ(mock, projectID, "az-1", `{"cpu":"25"}`)
		mock.ExpectCommit()

		_, err := SetProjectAZQuota(context.Background(), organizationID, projectID, "az-1", model.QuotaResources{"cpu": "25"})
		require.NoError(t, err)
		// The lowered quota is pushed before the commit.
		assert.Equal(t, []map[string]string{{"requests.cpu": "25", "limits.cpu": "25"}}, *pushedQuotas)
	})

	t.Run("lowering is rolled back when the AZ refuses or cannot apply it", func(t *testing.T) {
		setupAZs(t, "az-1", "az-2")
		setupUsage(t, AZUsage{Hard: map[string]int64{"cpu": 16000}, Used: map[string]int64{"cpu": 4000}}, nil)
		for pushError, expectedError := range map[error]error{
			fmt.Errorf("%w in az-1", ErrBelowUsage): ErrBelowUsage, // admitted since the usage was read
			assert.AnError:                          ErrAZUnavailable,
		} {
			setupPush(t, pushError)
			mock := setupMockDB(t)
			expectProjectTransaction(mock, organizationID, projectID)
			expectAZHierarchy(mock, "", "", `{"cpu":"16"}`, `{}`)
			expectUpsertAZQ(mock, projectID, "az-1", `{"cpu":"8"}`)
			mock.ExpectRollback()

			_, err := SetProjectAZQuota(context.Background(), organizationID, projectID, "az-1", model.QuotaResources{"cpu": "8"})
			require.ErrorIs(t, err, expectedError)
			assert.NotContains(t, err.Error(), assert.AnError.Error())
		}
	})

	t.Run("AZ quota above the project (stored before) cannot be increased", func(t *testing.T) {
		setupAZs(t, "az-1", "az-2")
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectAZHierarchy(mock, "", `{"cpu":"32"}`, `{"cpu":"30"}`, `{"cpu":"20"}`)
		mock.ExpectRollback()

		_, err := SetProjectAZQuota(context.Background(), organizationID, projectID, "az-1", model.QuotaResources{"cpu": "31"})
		require.ErrorIs(t, err, ErrProjectQuotaExceeded)
	})

	t.Run("resource not limited by the project", func(t *testing.T) {
		setupAZs(t, "az-1")
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectAZHierarchy(mock, "", `{"cpu":"32"}`, `{}`, `{}`)
		expectUpsertAZQ(mock, projectID, "az-1", `{"cpu":"1","memory":"1Ti"}`)
		mock.ExpectCommit()

		_, err := SetProjectAZQuota(context.Background(), organizationID, projectID, "az-1", model.QuotaResources{"cpu": "1", "memory": "1Ti"})
		require.NoError(t, err)
	})

	t.Run("resource limited by the organization but omitted by the project quota", func(t *testing.T) {
		setupAZs(t, "az-1")
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectAZHierarchy(mock, `{"cpu":"64","memory":"64Gi"}`, `{"cpu":"32"}`, `{}`, `{}`)
		mock.ExpectRollback()

		// The project quota omits memory, so the project gets no memory of the organization.
		_, err := SetProjectAZQuota(context.Background(), organizationID, projectID, "az-1", model.QuotaResources{"memory": "1Gi"})
		require.ErrorIs(t, err, ErrProjectQuotaExceeded)
		assert.Contains(t, err.Error(), "memory")
	})

	t.Run("project without quota", func(t *testing.T) {
		setupAZs(t, "az-1")
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectAZHierarchy(mock, "", "", `{}`, `{}`)
		expectUpsertAZQ(mock, projectID, "az-1", `{"memory":"1Ti"}`)
		mock.ExpectCommit()

		_, err := SetProjectAZQuota(context.Background(), organizationID, projectID, "az-1", model.QuotaResources{"memory": "1Ti"})
		require.NoError(t, err)
	})

	t.Run("project without quota in a limited organization", func(t *testing.T) {
		setupAZs(t, "az-1", "az-2")
		mock := setupMockDB(t)

		// Resources the organization does not limit are free.
		expectProjectTransaction(mock, organizationID, projectID)
		expectAZHierarchy(mock, `{"cpu":"64"}`, "", `{}`, `{}`)
		expectUpsertAZQ(mock, projectID, "az-1", `{"memory":"1Ti"}`)
		mock.ExpectCommit()

		_, err := SetProjectAZQuota(context.Background(), organizationID, projectID, "az-1", model.QuotaResources{"memory": "1Ti"})
		require.NoError(t, err)
	})

	t.Run("project without quota cannot take resources limited by the organization", func(t *testing.T) {
		setupAZs(t, "az-1", "az-2")
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectAZHierarchy(mock, `{"cpu":"64"}`, "", `{}`, `{}`)
		mock.ExpectRollback()

		_, err := SetProjectAZQuota(context.Background(), organizationID, projectID, "az-1", model.QuotaResources{"cpu": "1"})
		require.ErrorIs(t, err, ErrProjectQuotaExceeded)
	})

	t.Run("cannot be lowered below what is used", func(t *testing.T) {
		setupAZs(t, "az-1", "az-2")
		setupUsage(t, AZUsage{Hard: map[string]int64{"cpu": 16000, "memory": 8 << 30}, Used: map[string]int64{"cpu": 12000, "memory": 1 << 30}}, nil)
		setupPush(t, nil)

		for requestedCPU, expectedError := range map[string]error{"11": ErrBelowUsage, "12": nil, "100m": ErrBelowUsage} {
			mock := setupMockDB(t)
			expectProjectTransaction(mock, organizationID, projectID)
			expectAZHierarchy(mock, "", "", `{"cpu":"16","memory":"8Gi"}`, `{}`)
			if expectedError == nil {
				expectUpsertAZQ(mock, projectID, "az-1", `{"cpu":"12","memory":"8Gi"}`)
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}

			_, err := SetProjectAZQuota(context.Background(), organizationID, projectID, "az-1", model.QuotaResources{"cpu": requestedCPU, "memory": "8Gi"})
			if expectedError == nil {
				require.NoError(t, err, requestedCPU)
			} else {
				require.ErrorIs(t, err, expectedError, requestedCPU)
				assert.Contains(t, err.Error(), "cpu 12 used in az-1")
			}
		}
	})

	t.Run("removing a resource limited by a parent lowers it to 0", func(t *testing.T) {
		setupAZs(t, "az-1", "az-2")
		setupUsage(t, AZUsage{Hard: map[string]int64{"cpu": 16000}, Used: map[string]int64{"cpu": 1}}, nil)
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectAZHierarchy(mock, "", `{"cpu":"32"}`, `{"cpu":"16"}`, `{}`)
		mock.ExpectRollback()

		_, err := SetProjectAZQuota(context.Background(), organizationID, projectID, "az-1", model.QuotaResources{})
		require.ErrorIs(t, err, ErrBelowUsage)
	})

	t.Run("usage unavailable or not tracked", func(t *testing.T) {
		setupAZs(t, "az-1", "az-2")
		for name, usage := range map[string]AZUsage{
			"unreachable":                 {},
			"not enforced by the cluster": {Hard: map[string]int64{}, Used: map[string]int64{}},
			"not reported yet":            {Hard: map[string]int64{"cpu": 16000}, Used: map[string]int64{}},
		} {
			var readError error
			if name == "unreachable" {
				readError = assert.AnError
			}
			setupUsage(t, usage, readError)
			mock := setupMockDB(t)
			expectProjectTransaction(mock, organizationID, projectID)
			expectAZHierarchy(mock, "", "", `{"cpu":"16"}`, `{}`)
			mock.ExpectRollback()

			_, err := SetProjectAZQuota(context.Background(), organizationID, projectID, "az-1", model.QuotaResources{"cpu": "8"})
			require.ErrorIs(t, err, ErrUsageUnavailable, name)
			assert.NotContains(t, err.Error(), assert.AnError.Error(), name)
		}
	})

	t.Run("usage is not read when nothing is lowered", func(t *testing.T) {
		setupAZs(t, "az-1", "az-2")
		calledAZs := setupUsage(t, AZUsage{}, assert.AnError)
		pushedQuotas := setupPush(t, assert.AnError)
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectAZHierarchy(mock, "", "", `{"cpu":"16"}`, `{}`)
		expectUpsertAZQ(mock, projectID, "az-1", `{"cpu":"20","memory":"1Gi"}`)
		mock.ExpectCommit()

		// cpu is raised and memory limited for the first time.
		_, err := SetProjectAZQuota(context.Background(), organizationID, projectID, "az-1", model.QuotaResources{"cpu": "20", "memory": "1Gi"})
		require.NoError(t, err)
		assert.Empty(t, *calledAZs)
		assert.Empty(t, *pushedQuotas, "only lowered quotas are pushed before the commit")
	})

	t.Run("unknown or non whitelisted AZ", func(t *testing.T) {
		setupMockDB(t)
		config.Global.AZs = map[string]config.AZConfig{"az-1": {Code: "az-1", Whitelist: []string{uuid.NewString()}}}
		t.Cleanup(func() { config.Global.AZs = nil })

		for _, code := range []string{"az-1", "az-unknown"} {
			_, err := SetProjectAZQuota(context.Background(), organizationID, projectID, code, model.QuotaResources{"cpu": "1"})
			require.ErrorIs(t, err, ErrAZNotFound)
		}
	})
}

// ---------------------------------------------------------------------------------------------
// DeleteProjectQuota
// ---------------------------------------------------------------------------------------------

func TestDeleteProjectQuota(t *testing.T) {
	organizationID, projectID := uuid.New(), uuid.New()

	t.Run("deletes the project and AZ quotas", func(t *testing.T) {
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectOrgQuota(mock, organizationID, "")
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "project_az_quotas" WHERE project_id = $1`)).WithArgs(projectID).WillReturnResult(sqlmock.NewResult(0, 2))
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "project_quotas" WHERE project_id = $1`)).WithArgs(projectID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		require.NoError(t, DeleteProjectQuota(context.Background(), organizationID, projectID))
	})

	t.Run("refused while the organization has a quota", func(t *testing.T) {
		mock := setupMockDB(t)

		expectProjectTransaction(mock, organizationID, projectID)
		expectOrgQuota(mock, organizationID, `{"cpu":"64"}`)
		mock.ExpectRollback()

		require.ErrorIs(t, DeleteProjectQuota(context.Background(), organizationID, projectID), ErrProjectQuotaEnforced)
	})
}
