package user

import (
	"regexp"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMockDB(t *testing.T) (sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %s", err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm: %s", err)
	}

	oldClient := db.Client
	db.Client = gormDB

	return mock, func() {
		db.Client = oldClient
		sqlDB.Close()
	}
}

func TestFindWithOrganization(t *testing.T) {
	userID := uuid.New()
	orgID1 := uuid.New()
	orgID2 := uuid.New()
	ownerID := uuid.New()
	now := time.Now()

	userColumns := []string{
		"id", "created_at", "updated_at", "deleted_at",
		"firstname", "lastname", "email",
		"provider", "provider_id",
		"invite_code", "invite_code_regenerated_at", "is_active",
	}
	orgColumns := []string{
		"id", "created_at", "updated_at", "deleted_at",
		"name", "owner_id",
		"administrative_contact", "billing_contact", "technical_contact",
	}

	tests := []struct {
		name          string
		userId        string
		mockBehavior  func(mock sqlmock.Sqlmock)
		expectError   bool
		expectedOrgs  int
		expectedEmail string
	}{
		{
			name:   "User with multiple distinct guest orgs",
			userId: userID.String(),
			mockBehavior: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users"`)).
					WillReturnRows(sqlmock.NewRows(userColumns).
						AddRow(userID, now, now, nil, nil, nil, "user@test.com", "Kratos", "kratos-1", uuid.New(), nil, true))

				// PersonalOrg preload
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organizations"`)).
					WillReturnRows(sqlmock.NewRows(orgColumns))

				// DISTINCT query on organizations — deduplicates at DB level
 			mock.ExpectQuery(`SELECT DISTINCT`).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).
						AddRow(orgID1, "Org1", ownerID).
						AddRow(orgID2, "Org2", ownerID))
			},
			expectError:   false,
			expectedOrgs:  2,
			expectedEmail: "user@test.com",
		},
		{
			name:   "User with single guest org and single role",
			userId: userID.String(),
			mockBehavior: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users"`)).
					WillReturnRows(sqlmock.NewRows(userColumns).
						AddRow(userID, now, now, nil, nil, nil, "single@test.com", "Kratos", "kratos-5", uuid.New(), nil, true))

				// PersonalOrg preload
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organizations"`)).
					WillReturnRows(sqlmock.NewRows(orgColumns))

 			mock.ExpectQuery(`SELECT DISTINCT`).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).
						AddRow(orgID1, "Org1", ownerID))
			},
			expectError:   false,
			expectedOrgs:  1,
			expectedEmail: "single@test.com",
		},
		{
			name:   "DB DISTINCT deduplicates orgs when user has multiple roles in same org",
			userId: userID.String(),
			mockBehavior: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users"`)).
					WillReturnRows(sqlmock.NewRows(userColumns).
						AddRow(userID, now, now, nil, nil, nil, "multi@test.com", "Kratos", "kratos-3", uuid.New(), nil, true))

				// PersonalOrg preload
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organizations"`)).
					WillReturnRows(sqlmock.NewRows(orgColumns))

				// DISTINCT query should return only 2 unique orgs, not 3
 			mock.ExpectQuery(`SELECT DISTINCT`).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).
						AddRow(orgID1, "Org1", ownerID).
						AddRow(orgID2, "Org2", ownerID))
			},
			expectError:   false,
			expectedOrgs:  2,
			expectedEmail: "multi@test.com",
		},
		{
			name:   "User with no guest orgs",
			userId: userID.String(),
			mockBehavior: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users"`)).
					WillReturnRows(sqlmock.NewRows(userColumns).
						AddRow(userID, now, now, nil, nil, nil, "solo@test.com", "Kratos", "kratos-2", uuid.New(), nil, true))

				// PersonalOrg preload
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organizations"`)).
					WillReturnRows(sqlmock.NewRows(orgColumns))

				// No guest orgs
 			mock.ExpectQuery(`SELECT DISTINCT`).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}))
			},
			expectError:   false,
			expectedOrgs:  0,
			expectedEmail: "solo@test.com",
		},
		{
			name:   "User not found",
			userId: uuid.New().String(),
			mockBehavior: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users"`)).
					WillReturnRows(sqlmock.NewRows(userColumns))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockBehavior(mock)

			result, err := FindWithOrganization(tt.userId)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedEmail, result.Email)
			assert.Len(t, result.GuestOrg, tt.expectedOrgs)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFindWithOrganizationByProvider(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()
	ownerID := uuid.New()
	now := time.Now()

	userColumns := []string{
		"id", "created_at", "updated_at", "deleted_at",
		"firstname", "lastname", "email",
		"provider", "provider_id",
		"invite_code", "invite_code_regenerated_at", "is_active",
	}
	orgColumns := []string{
		"id", "created_at", "updated_at", "deleted_at",
		"name", "owner_id",
		"administrative_contact", "billing_contact", "technical_contact",
	}

	tests := []struct {
		name         string
		providerId   string
		provider     string
		mockBehavior func(mock sqlmock.Sqlmock)
		expectError  bool
		expectedOrgs int
	}{
		{
			name:       "Found by provider with one guest org",
			providerId: "kratos-1",
			provider:   "Kratos",
			mockBehavior: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users"`)).
					WillReturnRows(sqlmock.NewRows(userColumns).
						AddRow(userID, now, now, nil, nil, nil, "user@test.com", "Kratos", "kratos-1", uuid.New(), nil, true))

				// PersonalOrg preload
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organizations"`)).
					WillReturnRows(sqlmock.NewRows(orgColumns))

 			mock.ExpectQuery(`SELECT DISTINCT`).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).
						AddRow(orgID, "Org1", ownerID))
			},
			expectError:  false,
			expectedOrgs: 1,
		},
		{
			name:       "Provider not found",
			providerId: "unknown",
			provider:   "Kratos",
			mockBehavior: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users"`)).
					WillReturnRows(sqlmock.NewRows(userColumns))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockBehavior(mock)

			result, err := FindWithOrganizationByProvider(tt.providerId, tt.provider)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, result.GuestOrg, tt.expectedOrgs)

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
