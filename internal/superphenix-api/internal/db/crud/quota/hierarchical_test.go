package quota

import (
	"context"
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
		sqlDB.Close()
	}
}

func TestParseResources(t *testing.T) {
	tests := []struct {
		name      string
		input     model.QuotaResources
		expected  ParsedResources
		expectErr bool
	}{
		{
			name: "valid standard units",
			input: model.QuotaResources{
				CPU:    "16",
				Memory: "64Gi",
				Disk:   "1Ti",
			},
			expected: ParsedResources{
				MilliCPU: 16000,
				Memory:   64 * 1024 * 1024 * 1024,
				Disk:     1024 * 1024 * 1024 * 1024,
			},
			expectErr: false,
		},
		{
			name: "valid millicores and megabytes",
			input: model.QuotaResources{
				CPU:    "2500m",
				Memory: "512Mi",
				Disk:   "100Gi",
			},
			expected: ParsedResources{
				MilliCPU: 2500,
				Memory:   512 * 1024 * 1024,
				Disk:     100 * 1024 * 1024 * 1024,
			},
			expectErr: false,
		},
		{
			name: "empty resources",
			input: model.QuotaResources{
				CPU:    "",
				Memory: "",
				Disk:   "",
			},
			expected: ParsedResources{
				MilliCPU: 0,
				Memory:   0,
				Disk:     0,
			},
			expectErr: false,
		},
		{
			name: "invalid cpu format",
			input: model.QuotaResources{
				CPU: "invalid",
			},
			expectErr: true,
		},
		{
			name: "invalid memory format",
			input: model.QuotaResources{
				Memory: "not-a-number",
			},
			expectErr: true,
		},
		{
			name: "negative cpu",
			input: model.QuotaResources{
				CPU: "-4",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseResources(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, parsed)
			}
		})
	}
}

func TestSplitEqually(t *testing.T) {
	orig := ParsedResources{
		MilliCPU: 10000,
		Memory:   300 * 1024 * 1024 * 1024, // 300Gi
		Disk:     3 * 1024 * 1024 * 1024 * 1024,
	}

	// Split by 3
	split3 := orig.SplitEqually(3)
	assert.Equal(t, int64(3333), split3.MilliCPU)
	assert.Equal(t, int64(100*1024*1024*1024), split3.Memory)
	assert.Equal(t, int64(1024*1024*1024*1024), split3.Disk)

	// Ensure floor division never exceeds parent (sum of 3 parts <= parent)
	sum3 := split3.Add(split3).Add(split3)
	assert.True(t, sum3.MilliCPU <= orig.MilliCPU)
	assert.True(t, sum3.Memory <= orig.Memory)
	assert.True(t, sum3.Disk <= orig.Disk)

	// Split by 0 or negative
	assert.Equal(t, ParsedResources{}, orig.SplitEqually(0))
	assert.Equal(t, ParsedResources{}, orig.SplitEqually(-1))
}

func TestToQuotaResources(t *testing.T) {
	p := ParsedResources{
		MilliCPU: 16000,
		Memory:   64 * 1024 * 1024 * 1024,
		Disk:     1024 * 1024 * 1024 * 1024,
	}
	r := p.ToQuotaResources()
	assert.Equal(t, "16", r.CPU)
	assert.Equal(t, "64Gi", r.Memory)
	assert.Equal(t, "1Ti", r.Disk)

	pOdd := ParsedResources{
		MilliCPU: 3500,
		Memory:   500 * 1024 * 1024,
		Disk:     50 * 1024 * 1024 * 1024,
	}
	rOdd := pOdd.ToQuotaResources()
	assert.Equal(t, "3500m", rOdd.CPU)
	assert.Equal(t, "500Mi", rOdd.Memory)
	assert.Equal(t, "50Gi", rOdd.Disk)
}

func TestExceeds(t *testing.T) {
	limit := ParsedResources{
		MilliCPU: 16000,
		Memory:   64 * 1024 * 1024 * 1024,
		Disk:     1024 * 1024 * 1024 * 1024,
	}

	under := ParsedResources{
		MilliCPU: 8000,
		Memory:   32 * 1024 * 1024 * 1024,
		Disk:     512 * 1024 * 1024 * 1024,
	}
	exceeded, _ := under.Exceeds(limit)
	assert.False(t, exceeded)

	exact := limit
	exceeded, _ = exact.Exceeds(limit)
	assert.False(t, exceeded)

	overCPU := ParsedResources{
		MilliCPU: 17000,
		Memory:   32 * 1024 * 1024 * 1024,
		Disk:     512 * 1024 * 1024 * 1024,
	}
	exceeded, msg := overCPU.Exceeds(limit)
	assert.True(t, exceeded)
	assert.Contains(t, msg, "CPU quota exceeded")

	overMem := ParsedResources{
		MilliCPU: 8000,
		Memory:   65 * 1024 * 1024 * 1024,
		Disk:     512 * 1024 * 1024 * 1024,
	}
	exceeded, msg = overMem.Exceeds(limit)
	assert.True(t, exceeded)
	assert.Contains(t, msg, "Memory quota exceeded")

	overDisk := ParsedResources{
		MilliCPU: 8000,
		Memory:   32 * 1024 * 1024 * 1024,
		Disk:     2048 * 1024 * 1024 * 1024,
	}
	exceeded, msg = overDisk.Exceeds(limit)
	assert.True(t, exceeded)
	assert.Contains(t, msg, "Disk quota exceeded")
}

func TestGetOrganizationQuota(t *testing.T) {
	orgID := uuid.New()

	t.Run("found", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organization_quotas" WHERE organization_id = $1 AND "organization_quotas"."deleted_at" IS NULL ORDER BY "organization_quotas"."organization_id" LIMIT $2`)).
			WithArgs(orgID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"organization_id", "resources"}).
				AddRow(orgID, `{"cpu":"16","memory":"64Gi","disk":"1Ti"}`))

		oq, err := GetOrganizationQuota(context.Background(), orgID)
		require.NoError(t, err)
		require.NotNil(t, oq)
		assert.Equal(t, "16", oq.Resources.CPU)
	})

	t.Run("unlimited (not found)", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organization_quotas" WHERE organization_id = $1 AND "organization_quotas"."deleted_at" IS NULL ORDER BY "organization_quotas"."organization_id" LIMIT $2`)).
			WithArgs(orgID, 1).
			WillReturnError(gorm.ErrRecordNotFound)

		oq, err := GetOrganizationQuota(context.Background(), orgID)
		require.NoError(t, err)
		assert.Nil(t, oq)
	})
}

func TestSetOrganizationQuota_AutoSplit(t *testing.T) {
	orgID := uuid.New()
	proj1 := uuid.New()
	proj2 := uuid.New()

	config.Global.AZs = map[string]config.AZConfig{
		"az-1": {Code: "az-1"},
		"az-2": {Code: "az-2"},
	}

	mock, cleanup := setupMockDB(t)
	defer cleanup()

	mock.ExpectBegin()
	// Find projects under org
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "projects" WHERE orga_id = $1 AND "projects"."deleted_at" IS NULL`)).
		WithArgs(orgID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "orga_id", "name"}).
			AddRow(proj1, orgID, "Proj 1").
			AddRow(proj2, orgID, "Proj 2"))

	// Find existing project quotas
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas" WHERE organization_id = $1 AND "project_quotas"."deleted_at" IS NULL`)).
		WithArgs(orgID).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "organization_id", "resources"}))

	// Project 1 check
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas" WHERE project_id = $1 AND "project_quotas"."deleted_at" IS NULL ORDER BY "project_quotas"."project_id" LIMIT $2`)).
		WithArgs(proj1, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	// Project 1 quota insert
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "project_quotas"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Project 1 AZ1 find and insert
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas" WHERE (project_id = $1 AND code_az = $2) AND "project_az_quotas"."deleted_at" IS NULL`)).
		WithArgs(proj1, "az-1", 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "project_az_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	// Project 1 AZ2 find and insert
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas" WHERE (project_id = $1 AND code_az = $2) AND "project_az_quotas"."deleted_at" IS NULL`)).
		WithArgs(proj1, "az-2", 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "project_az_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	// Project 2 check
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas" WHERE project_id = $1 AND "project_quotas"."deleted_at" IS NULL ORDER BY "project_quotas"."project_id" LIMIT $2`)).
		WithArgs(proj2, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	// Project 2 quota insert
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "project_quotas"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Project 2 AZ1 find and insert
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas" WHERE (project_id = $1 AND code_az = $2) AND "project_az_quotas"."deleted_at" IS NULL`)).
		WithArgs(proj2, "az-1", 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "project_az_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	// Project 2 AZ2 find and insert
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas" WHERE (project_id = $1 AND code_az = $2) AND "project_az_quotas"."deleted_at" IS NULL`)).
		WithArgs(proj2, "az-2", 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "project_az_quotas"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	// Org quota check and save
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organization_quotas" WHERE organization_id = $1 AND "organization_quotas"."deleted_at" IS NULL ORDER BY "organization_quotas"."organization_id" LIMIT $2`)).
		WithArgs(orgID, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "organization_quotas"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	oq, err := SetOrganizationQuota(context.Background(), orgID, model.QuotaResources{
		CPU:    "64",
		Memory: "256Gi",
		Disk:   "2Ti",
	})
	require.NoError(t, err)
	require.NotNil(t, oq)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetProjectQuota(t *testing.T) {
	orgID := uuid.New()
	projID := uuid.New()
	otherProjID := uuid.New()

	config.Global.AZs = map[string]config.AZConfig{
		"az-1": {Code: "az-1"},
	}

	t.Run("success within org quota and auto splits to AZs", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()

		mock.ExpectBegin()
		// Find org quota
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organization_quotas" WHERE organization_id = $1 AND "organization_quotas"."deleted_at" IS NULL ORDER BY "organization_quotas"."organization_id" LIMIT $2`)).
			WithArgs(orgID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"organization_id", "resources"}).
				AddRow(orgID, `{"cpu":"64","memory":"256Gi","disk":"2Ti"}`))

		// Find other project quotas
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas" WHERE (organization_id = $1 AND project_id != $2) AND "project_quotas"."deleted_at" IS NULL`)).
			WithArgs(orgID, projID).
			WillReturnRows(sqlmock.NewRows([]string{"project_id", "organization_id", "resources"}).
				AddRow(otherProjID, orgID, `{"cpu":"32","memory":"128Gi","disk":"1Ti"}`))

		// Check project quota
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas" WHERE project_id = $1 AND "project_quotas"."deleted_at" IS NULL ORDER BY "project_quotas"."project_id" LIMIT $2`)).
			WithArgs(projID, 1).
			WillReturnError(gorm.ErrRecordNotFound)
		// Save project quota
		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "project_quotas"`)).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// AZ quota find and insert
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas" WHERE (project_id = $1 AND code_az = $2) AND "project_az_quotas"."deleted_at" IS NULL`)).
			WithArgs(projID, "az-1", 1).
			WillReturnError(gorm.ErrRecordNotFound)
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "project_az_quotas"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

		mock.ExpectCommit()

		pq, err := SetProjectQuota(context.Background(), orgID, projID, model.QuotaResources{
			CPU:    "32",
			Memory: "128Gi",
			Disk:   "1Ti",
		})
		require.NoError(t, err)
		require.NotNil(t, pq)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("exceeds org quota", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organization_quotas" WHERE organization_id = $1 AND "organization_quotas"."deleted_at" IS NULL ORDER BY "organization_quotas"."organization_id" LIMIT $2`)).
			WithArgs(orgID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"organization_id", "resources"}).
				AddRow(orgID, `{"cpu":"64","memory":"256Gi","disk":"2Ti"}`))

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas" WHERE (organization_id = $1 AND project_id != $2) AND "project_quotas"."deleted_at" IS NULL`)).
			WithArgs(orgID, projID).
			WillReturnRows(sqlmock.NewRows([]string{"project_id", "organization_id", "resources"}).
				AddRow(otherProjID, orgID, `{"cpu":"48","memory":"128Gi","disk":"1Ti"}`))

		mock.ExpectRollback()

		_, err := SetProjectQuota(context.Background(), orgID, projID, model.QuotaResources{
			CPU:    "32",
			Memory: "128Gi",
			Disk:   "1Ti",
		})
		require.ErrorIs(t, err, ErrOrgQuotaExceeded)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSetProjectAZQuota(t *testing.T) {
	projID := uuid.New()

	t.Run("success within project quota", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas" WHERE project_id = $1 AND "project_quotas"."deleted_at" IS NULL ORDER BY "project_quotas"."project_id" LIMIT $2`)).
			WithArgs(projID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"project_id", "resources"}).
				AddRow(projID, `{"cpu":"32","memory":"128Gi","disk":"1Ti"}`))

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas" WHERE (project_id = $1 AND code_az != $2) AND "project_az_quotas"."deleted_at" IS NULL`)).
			WithArgs(projID, "az-1").
			WillReturnRows(sqlmock.NewRows([]string{"project_id", "code_az", "resources"}).
				AddRow(projID, "az-2", `{"cpu":"16","memory":"64Gi","disk":"500Gi"}`))

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas" WHERE (project_id = $1 AND code_az = $2) AND "project_az_quotas"."deleted_at" IS NULL`)).
			WithArgs(projID, "az-1", 1).
			WillReturnError(gorm.ErrRecordNotFound)

		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "project_az_quotas"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

		mock.ExpectCommit()

		azq, err := SetProjectAZQuota(context.Background(), projID, "az-1", model.QuotaResources{
			CPU:    "16",
			Memory: "64Gi",
			Disk:   "500Gi",
		})
		require.NoError(t, err)
		require.NotNil(t, azq)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("exceeds project quota", func(t *testing.T) {
		mock, cleanup := setupMockDB(t)
		defer cleanup()

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_quotas" WHERE project_id = $1 AND "project_quotas"."deleted_at" IS NULL ORDER BY "project_quotas"."project_id" LIMIT $2`)).
			WithArgs(projID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"project_id", "resources"}).
				AddRow(projID, `{"cpu":"32","memory":"128Gi","disk":"1Ti"}`))

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "project_az_quotas" WHERE (project_id = $1 AND code_az != $2) AND "project_az_quotas"."deleted_at" IS NULL`)).
			WithArgs(projID, "az-1").
			WillReturnRows(sqlmock.NewRows([]string{"project_id", "code_az", "resources"}).
				AddRow(projID, "az-2", `{"cpu":"20","memory":"64Gi","disk":"500Gi"}`))

		mock.ExpectRollback()

		_, err := SetProjectAZQuota(context.Background(), projID, "az-1", model.QuotaResources{
			CPU:    "16",
			Memory: "64Gi",
			Disk:   "500Gi",
		})
		require.ErrorIs(t, err, ErrProjectQuotaExceeded)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
