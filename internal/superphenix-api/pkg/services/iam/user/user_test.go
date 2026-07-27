package user

import (
	"context"
	"encoding/json"
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

func TestRegenerateInviteCode(t *testing.T) {
	mockDb, mock, _ := sqlmock.New()
	dialector := postgres.New(postgres.Config{
		Conn:       mockDb,
		DriverName: "postgres",
	})
	client, _ := gorm.Open(dialector, &gorm.Config{})
	db.Client = client

	userId := uuid.New()

	type testCase struct {
		name           string
		userId         interface{}
		setupMock      func(mock sqlmock.Sqlmock)
		expectedStatus int
		verifyResponse func(t *testing.T, rr *httptest.ResponseRecorder)
	}

	tests := []testCase{
		{
			name:   "Success - first regeneration (null timestamp)",
			userId: userId.String(),
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "invite_code_regenerated_at"}).
					AddRow(userId, nil)
				mock.ExpectQuery(`SELECT .+ FROM "users"`).
					WithArgs(userId, 1).
					WillReturnRows(rows)
				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE "users" SET`).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), userId).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp RegenerateInviteCodeResponse
				err := json.Unmarshal(rr.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, resp.InviteCode)
			},
		},
		{
			name:   "Success - regeneration after cooldown",
			userId: userId.String(),
			setupMock: func(mock sqlmock.Sqlmock) {
				oldTime := time.Now().Add(-25 * time.Hour)
				rows := sqlmock.NewRows([]string{"id", "invite_code_regenerated_at"}).
					AddRow(userId, oldTime)
				mock.ExpectQuery(`SELECT .+ FROM "users"`).
					WithArgs(userId, 1).
					WillReturnRows(rows)
				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE "users" SET`).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), userId).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp RegenerateInviteCodeResponse
				err := json.Unmarshal(rr.Body.Bytes(), &resp)
				assert.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, resp.InviteCode)
			},
		},
		{
			name:   "Cooldown not elapsed",
			userId: userId.String(),
			setupMock: func(mock sqlmock.Sqlmock) {
				recentTime := time.Now().Add(-1 * time.Hour)
				rows := sqlmock.NewRows([]string{"id", "invite_code_regenerated_at"}).
					AddRow(userId, recentTime)
				mock.ExpectQuery(`SELECT .+ FROM "users"`).
					WithArgs(userId, 1).
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "No user in context",
			userId:         nil,
			setupMock:      func(mock sqlmock.Sqlmock) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:   "User not found",
			userId: userId.String(),
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM "users"`).
					WithArgs(userId, 1).
					WillReturnError(gorm.ErrRecordNotFound)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "Database error on update",
			userId: userId.String(),
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "invite_code_regenerated_at"}).
					AddRow(userId, nil)
				mock.ExpectQuery(`SELECT .+ FROM "users"`).
					WithArgs(userId, 1).
					WillReturnRows(rows)
				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE "users" SET`).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), userId).
					WillReturnError(gorm.ErrInvalidDB)
				mock.ExpectRollback()
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(mock)

			req := httptest.NewRequest(http.MethodPost, "/v1/invite-code", nil)
			if tt.userId != nil {
				req = req.WithContext(context.WithValue(req.Context(), consts.ContextUserId, tt.userId))
			}

			rr := httptest.NewRecorder()
			New(nil).RegenerateInviteCode(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.verifyResponse != nil {
				tt.verifyResponse(t, rr)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
