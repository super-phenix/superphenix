package apiToken

import (
	"net/http"
	"net/http/httptest"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestApiTokenAuth(t *testing.T) {
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
	prefix := "abcdef12"
	salt := "somesaltvalue"
	token := prefix + "tokencontent"
	hashedToken := hashToken(token, salt)

	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		mockBehavior   func()
		expectedError  string
		checkContext   bool
		expectedResult bool // For Detection
	}{
		{
			name: "Detection - No token",
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/", nil)
			},
			mockBehavior:   func() {},
			expectedResult: false,
		},
		{
			name: "Detection - Valid keyword",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Authorization", "api-token some-token")
				return req
			},
			mockBehavior:   func() {},
			expectedResult: true,
		},
		{
			name: "Validation - Token too short",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Authorization", "api-token short")
				return req
			},
			mockBehavior:  func() {},
			expectedError: "API Token invalid",
		},
		{
			name: "Validation - Token not found",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Authorization", "api-token "+token)
				return req
			},
			mockBehavior: func() {
				mock.ExpectQuery(`SELECT \* FROM "api_tokens"`).
					WithArgs(prefix).
					WillReturnError(gorm.ErrRecordNotFound)
			},
			expectedError: "record not found",
		},
		{
			name: "Validation - Hash mismatch",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Authorization", "api-token "+token)
				return req
			},
			mockBehavior: func() {
				rows := sqlmock.NewRows([]string{"id", "prefix", "salt", "token_encrypted", "user_id"}).
					AddRow(uuid.New(), prefix, salt, "wronghash", userID)
				mock.ExpectQuery(`SELECT \* FROM "api_tokens"`).
					WithArgs(prefix).
					WillReturnRows(rows)

				// Preload User
				userRows := sqlmock.NewRows([]string{"id", "is_active"}).
					AddRow(userID, true)
				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(userID).
					WillReturnRows(userRows)
			},
			expectedError: "API Token invalid",
		},
		{
			name: "Validation - User inactive",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Authorization", "api-token "+token)
				return req
			},
			mockBehavior: func() {
				rows := sqlmock.NewRows([]string{"id", "prefix", "salt", "token_encrypted", "user_id"}).
					AddRow(uuid.New(), prefix, salt, hashedToken, userID)
				mock.ExpectQuery(`SELECT \* FROM "api_tokens"`).
					WithArgs(prefix).
					WillReturnRows(rows)

				// Preload User
				userRows := sqlmock.NewRows([]string{"id", "is_active"}).
					AddRow(userID, false)
				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(userID).
					WillReturnRows(userRows)
			},
			expectedError: "API Token invalid",
		},
		{
			name: "Validation - Token expired",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Authorization", "api-token "+token)
				return req
			},
			mockBehavior: func() {
				rows := sqlmock.NewRows([]string{"id", "prefix", "salt", "token_encrypted", "user_id", "expires_at"}).
					AddRow(uuid.New(), prefix, salt, hashedToken, userID, time.Now().Add(-time.Hour))
				mock.ExpectQuery(`SELECT \* FROM "api_tokens"`).
					WithArgs(prefix).
					WillReturnRows(rows)

				userRows := sqlmock.NewRows([]string{"id", "is_active"}).
					AddRow(userID, true)
				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(userID).
					WillReturnRows(userRows)
			},
			expectedError: "API Token expired",
		},
		{
			name: "Validation - Success",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Authorization", "api-token "+token)
				return req
			},
			mockBehavior: func() {
				rows := sqlmock.NewRows([]string{"id", "prefix", "salt", "token_encrypted", "user_id", "expires_at"}).
					AddRow(uuid.New(), prefix, salt, hashedToken, userID, time.Now().Add(time.Hour))
				mock.ExpectQuery(`SELECT \* FROM "api_tokens"`).
					WithArgs(prefix).
					WillReturnRows(rows)

				userRows := sqlmock.NewRows([]string{"id", "is_active"}).
					AddRow(userID, true)
				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(userID).
					WillReturnRows(userRows)
			},
			checkContext: true,
		},
		{
			name: "Validation - Multiple tokens same prefix",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Authorization", "api-token "+token)
				return req
			},
			mockBehavior: func() {
				// First row is a mismatch
				// Second row matches
				otherUserID := uuid.New()
				rows := sqlmock.NewRows([]string{"id", "prefix", "salt", "token_encrypted", "user_id", "expires_at"}).
					AddRow(uuid.New(), prefix, "othersalt", "otherhash", otherUserID, time.Now().Add(time.Hour)).
					AddRow(uuid.New(), prefix, salt, hashedToken, userID, time.Now().Add(time.Hour))
				mock.ExpectQuery(`SELECT \* FROM "api_tokens"`).
					WithArgs(prefix).
					WillReturnRows(rows)

				// GORM uses IN query for preloading multiple records
				userRows := sqlmock.NewRows([]string{"id", "is_active"}).
					AddRow(otherUserID, true).
					AddRow(userID, true)

				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnRows(userRows)
			},
			checkContext: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockBehavior()
			req := tt.setupRequest()
			rr := httptest.NewRecorder()

			if tt.expectedError == "" && tt.name[:9] == "Detection" {
				got := ApiTokenAuth.Detection(rr, req)
				assert.Equal(t, tt.expectedResult, got)
			} else {
				newReq, err := ApiTokenAuth.Validation(rr, req)
				if tt.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tt.expectedError)
				} else {
					assert.NoError(t, err)
					if tt.checkContext {
						assert.Equal(t, userID.String(), newReq.Context().Value(consts.ContextUserId))
					}
				}
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
