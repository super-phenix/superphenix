package apiToken

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestHashToken(t *testing.T) {
	token := "my-secret-token"
	salt := "somesaltvalue"
	hashed := hashToken(token, salt)

	// SHA3-512 should produce a 128-character hex string (512 bits / 4 bits per hex char = 128)
	assert.Len(t, hashed, 128)

	// Re-hashing the same token should give the same result
	assert.Equal(t, hashed, hashToken(token, salt))

	// Different tokens should give different hashes
	assert.NotEqual(t, hashed, hashToken("different-token", salt))

	// Different salts should give different hashes
	assert.NotEqual(t, hashed, hashToken(token, "differentsalt"))
}

func TestVerifyToken(t *testing.T) {
	token := "my-secret-token"
	salt := "somesaltvalue"
	hashed := hashToken(token, salt)

	assert.True(t, verifyToken(token, salt, hashed))
	assert.False(t, verifyToken("wrong-token", salt, hashed))
	assert.False(t, verifyToken(token, "wrong-salt", hashed))
}

func TestHandlers(t *testing.T) {
	mockDb, mock, _ := sqlmock.New()
	dialector := postgres.New(postgres.Config{
		Conn:       mockDb,
		DriverName: "postgres",
	})
	client, _ := gorm.Open(dialector, &gorm.Config{})
	db.Client = client

	userId := uuid.New()
	otherUserId := uuid.New()
	tokenId := uuid.New()

	type testCase struct {
		name           string
		method         string
		url            string
		body           interface{}
		userId         string
		setupMock      func(mock sqlmock.Sqlmock)
		expectedStatus int
		verifyResponse func(t *testing.T, rr *httptest.ResponseRecorder)
	}

	tests := []testCase{
		{
			name:   "CreateAPIToken - Success",
			method: http.MethodPost,
			url:    "/api-token",
			body: createBody{
				Name:      "test-token",
				ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
			},
			userId: userId.String(),
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(`INSERT INTO "api_tokens" \("created_at","updated_at","deleted_at","name","prefix","user_id","expires_at","salt","token_encrypted"\)`).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "test-token", sqlmock.AnyArg(), userId, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
				mock.ExpectCommit()
			},
			expectedStatus: http.StatusCreated,
			verifyResponse: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp createResponse
				err := json.Unmarshal(rr.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.NotEmpty(t, resp.Token)
				assert.Len(t, resp.Token, 64)
			},
		},
		{
			name:   "CreateAPIToken - Expired",
			method: http.MethodPost,
			url:    "/api-token",
			body: createBody{
				Name:      "test-token",
				ExpiresAt: time.Now().Add(-time.Hour).Format(time.RFC3339),
			},
			userId:         userId.String(),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "RevokeAPIToken - Success",
			method: http.MethodDelete,
			url:    "/api-token/" + tokenId.String(),
			userId: userId.String(),
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "api_tokens" WHERE "api_tokens"\."id" = \$1 AND "api_tokens"\."deleted_at" IS NULL AND "api_tokens"\."id" = \$2 ORDER BY "api_tokens"\."id" LIMIT \$3`).
					WithArgs(tokenId, tokenId, 1).
					WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(tokenId, userId))

				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE "api_tokens" SET "deleted_at"=\$1 WHERE "api_tokens"\."id" = \$2 AND "api_tokens"\."deleted_at" IS NULL`).
					WithArgs(sqlmock.AnyArg(), tokenId).
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:   "RevokeAPIToken - Not Found",
			method: http.MethodDelete,
			url:    "/api-token/" + tokenId.String(),
			userId: userId.String(),
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "api_tokens" WHERE "api_tokens"\."id" = \$1 AND "api_tokens"\."deleted_at" IS NULL AND "api_tokens"\."id" = \$2 ORDER BY "api_tokens"\."id" LIMIT \$3`).
					WithArgs(tokenId, tokenId, 1).
					WillReturnError(gorm.ErrRecordNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "RevokeAPIToken - Forbidden",
			method: http.MethodDelete,
			url:    "/api-token/" + tokenId.String(),
			userId: userId.String(),
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "api_tokens" WHERE "api_tokens"\."id" = \$1 AND "api_tokens"\."deleted_at" IS NULL AND "api_tokens"\."id" = \$2 ORDER BY "api_tokens"\."id" LIMIT \$3`).
					WithArgs(tokenId, tokenId, 1).
					WillReturnRows(sqlmock.NewRows([]string{"id", "user_id"}).AddRow(tokenId, otherUserId))
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "ListAPIToken",
			method: http.MethodGet,
			url:    "/api-token",
			userId: userId.String(),
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "api_tokens" WHERE "api_tokens"\."user_id" = \$1 AND "api_tokens"\."deleted_at" IS NULL`).
					WithArgs(userId).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(uuid.New(), "test-token"))
			},
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var tokens []interface{}
				err := json.Unmarshal(rr.Body.Bytes(), &tokens)
				assert.NoError(t, err)
				assert.Len(t, tokens, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyReader *bytes.Buffer
			if tt.body != nil {
				jsonBody, _ := json.Marshal(tt.body)
				bodyReader = bytes.NewBuffer(jsonBody)
			} else {
				bodyReader = bytes.NewBuffer(nil)
			}

			req := httptest.NewRequest(tt.method, tt.url, bodyReader)
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}

			ctx := context.WithValue(req.Context(), consts.ContextUserId, tt.userId)
			if tt.method == http.MethodDelete {
				// Mock Chi router context for tokenId param
				idStr := tt.url[len("/api-token/"):]
				chiCtx := chi.NewRouteContext()
				chiCtx.URLParams.Add("tokenId", idStr)
				ctx = context.WithValue(ctx, chi.RouteCtxKey, chiCtx)
			}
			req = req.WithContext(ctx)
			rr := httptest.NewRecorder()

			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			switch tt.name {
			case "CreateAPIToken - Success", "CreateAPIToken - Expired":
				New(nil).CreateAPIToken(rr, req)
			case "ListAPIToken":
				New(nil).ListAPIToken(rr, req)
			default:
				if tt.method == http.MethodDelete {
					New(nil).RevokeAPIToken(rr, req)
				}
			}

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.verifyResponse != nil {
				tt.verifyResponse(t, rr)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
