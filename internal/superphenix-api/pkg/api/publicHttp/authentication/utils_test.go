package authentication

import (
	"context"
	"net/http/httptest"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRetrieveUserFromSession(t *testing.T) {
	// Setup sqlmock
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer sqlDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening gorm database", err)
	}

	// Override global db client
	oldClient := db.Client
	db.Client = gormDB
	defer func() { db.Client = oldClient }()

	userID := uuid.New()
	providerID := "kratos-id"

	tests := []struct {
		name          string
		setupContext  func() context.Context
		mockBehavior  func()
		expectedError string
		checkContext  bool
	}{
		{
			name: "No session in context",
			setupContext: func() context.Context {
				return context.Background()
			},
			mockBehavior:  func() {},
			expectedError: "no session found",
		},
		{
			name: "User not found in DB",
			setupContext: func() context.Context {
				return context.WithValue(context.Background(), consts.ContextSessionId, providerID)
			},
			mockBehavior: func() {
				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(db.KratosProvider, providerID, 1).
					WillReturnError(gorm.ErrRecordNotFound)
			},
			expectedError: "record not found",
		},
		{
			name: "User inactive",
			setupContext: func() context.Context {
				return context.WithValue(context.Background(), consts.ContextSessionId, providerID)
			},
			mockBehavior: func() {
				// First query for FindByProviderId
				rows := sqlmock.NewRows([]string{"id", "provider_id", "provider", "is_active"}).
					AddRow(userID, providerID, db.KratosProvider, false)
				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(db.KratosProvider, providerID, 1).
					WillReturnRows(rows)

				// Second query for IsUserActive
				activeRows := sqlmock.NewRows([]string{"id", "is_active"}).
					AddRow(userID, false)
				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(userID, userID, 1).
					WillReturnRows(activeRows)
			},
			expectedError: "user is not active",
		},
		{
			name: "User soft-deleted",
			setupContext: func() context.Context {
				return context.WithValue(context.Background(), consts.ContextSessionId, providerID)
			},
			mockBehavior: func() {
				// GORM by default adds "AND deleted_at IS NULL" to queries.
				// If a user is soft-deleted, First() will return ErrRecordNotFound.
				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(db.KratosProvider, providerID, 1).
					WillReturnError(gorm.ErrRecordNotFound)
			},
			expectedError: "record not found",
		},
		{
			name: "Success",
			setupContext: func() context.Context {
				return context.WithValue(context.Background(), consts.ContextSessionId, providerID)
			},
			mockBehavior: func() {
				// First query for FindByProviderId
				rows := sqlmock.NewRows([]string{"id", "provider_id", "provider", "is_active"}).
					AddRow(userID, providerID, db.KratosProvider, true)
				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(db.KratosProvider, providerID, 1).
					WillReturnRows(rows)

				// Second query for IsUserActive
				activeRows := sqlmock.NewRows([]string{"id", "is_active"}).
					AddRow(userID, true)
				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(userID, userID, 1).
					WillReturnRows(activeRows)
			},
			checkContext: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockBehavior()

			req := httptest.NewRequest("GET", "/", nil)
			req = req.WithContext(tt.setupContext())

			newReq, err := RetrieveUserFromSession(req)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				if tt.checkContext {
					val := newReq.Context().Value(consts.ContextUserId)
					assert.Equal(t, userID.String(), val)
				}
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
