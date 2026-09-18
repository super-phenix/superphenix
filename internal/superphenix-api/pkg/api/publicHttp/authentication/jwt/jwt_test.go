package jwt

import (
	"net/http"
	"net/http/httptest"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authentication/jwt"
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

func TestJwtBearerAuth(t *testing.T) {
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

	// Helper to create valid JWT
	createToken := func(aud string) string {
		var tokenStr string
		if aud == jwt.AccessAudience {
			tokenStr, _ = jwt.CreateAccessToken(time.Hour, providerID)
		} else {
			token := jwt.NewRefreshToken(time.Hour, providerID)
			tokenStr, _ = token.SignedString(jwt.Secret)
		}
		return tokenStr
	}

	validAccessToken := createToken(jwt.AccessAudience)
	validRefreshToken := createToken(jwt.RefreshAudience)

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
			name: "Detection - Header token",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Authorization", "Bearer token")
				return req
			},
			mockBehavior:   func() {},
			expectedResult: true,
		},
		{
			name: "Detection - URL token",
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/?bearer=token", nil)
			},
			mockBehavior:   func() {},
			expectedResult: true,
		},
		{
			name: "Detection - Whitespace-only header does not panic",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Authorization", "   ")
				return req
			},
			mockBehavior:   func() {},
			expectedResult: false,
		},
		{
			name: "Validation - Invalid token format",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Authorization", "Bearer invalid")
				return req
			},
			mockBehavior:  func() {},
			expectedError: "token is invalid",
		},
		{
			name: "Validation - Wrong audience",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Authorization", "Bearer "+validRefreshToken)
				return req
			},
			mockBehavior:  func() {},
			expectedError: "invalid audience",
		},
		{
			name: "Validation - Success",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("Authorization", "Bearer "+validAccessToken)
				return req
			},
			mockBehavior: func() {
				// RetrieveUserFromSession mocks
				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(db.KratosProvider, providerID, 1).
					WillReturnRows(sqlmock.NewRows([]string{"id", "provider_id", "provider", "is_active"}).
						AddRow(userID, providerID, db.KratosProvider, true))

				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(userID, userID, 1).
					WillReturnRows(sqlmock.NewRows([]string{"id", "is_active"}).
						AddRow(userID, true))
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
				got := JwtBearerAuth.Detection(rr, req)
				assert.Equal(t, tt.expectedResult, got)
			} else {
				newReq, err := JwtBearerAuth.Validation(rr, req)
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
