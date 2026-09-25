package auditEvent

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

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
		_ = sqlDB.Close()
	}
}

func TestInsert(t *testing.T) {
	userId := uuid.New()

	tests := []struct {
		name      string
		event     model.AuditEvent
		mockSetup func(mock sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name:  "resolves the email of a known user",
			event: model.AuditEvent{UserId: &userId, EventType: "disk.create", Status: model.AuditStatusAttempted},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`INSERT INTO "audit_events" .*SELECT email FROM users WHERE id = `).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
		},
		{
			name:  "leaves the email empty without a user",
			event: model.AuditEvent{EventType: "session.login", Status: model.AuditStatusAttempted},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`INSERT INTO "audit_events"`).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
		},
		{
			name:  "propagates the insert error",
			event: model.AuditEvent{EventType: "disk.create"},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`INSERT INTO "audit_events"`).WillReturnError(errors.New("boom"))
				mock.ExpectRollback()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			id, err := Insert(context.Background(), tt.event)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, id)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFinalize(t *testing.T) {
	projectId := uuid.New()
	resourceId := "spx-1"

	tests := []struct {
		name       string
		completion Completion
		wantSQL    string
	}{
		{
			name:       "writes the outcome only",
			completion: Completion{Status: model.AuditStatusSuccess, StatusCode: 200},
			wantSQL:    `UPDATE "audit_events" SET "completed_at"=\$1,"status"=\$2,"status_code"=\$3 WHERE id = \$4`,
		},
		{
			name: "writes the project and resource found by the handler",
			completion: Completion{
				Status: model.AuditStatusFailed, StatusCode: 500, ProjectId: &projectId, ResourceId: &resourceId,
			},
			wantSQL: `UPDATE "audit_events" SET "completed_at"=\$1,"project_id"=\$2,"resource_id"=\$3,` +
				`"status"=\$4,"status_code"=\$5 WHERE id = \$6`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			mock.ExpectBegin()
			mock.ExpectExec(tt.wantSQL).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			assert.NoError(t, Finalize(context.Background(), uuid.New(), tt.completion))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestList(t *testing.T) {
	orgId := uuid.New()
	userId := uuid.New()
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	tests := []struct {
		name      string
		filter    Filter
		wantWhere string
		wantArgs  int
		queryErr  error
		wantErr   bool
	}{
		{
			name:      "organization page",
			filter:    Filter{OrganizationId: &orgId, Limit: 25},
			wantWhere: `WHERE organization_id = \$1`,
			wantArgs:  1,
		},
		{
			name: "every filter",
			filter: Filter{
				OrganizationId: &orgId, ProjectId: &orgId, UserId: &userId, UserEmail: "A@b.c",
				EventTypes: []string{"disk.create", "disk.delete"}, ResourceType: "disk", ResourceId: "spx-1",
				Status: model.AuditStatusFailed, From: &from, To: &to, Limit: 10, Offset: 20,
			},
			wantWhere: `WHERE organization_id = \$1 AND project_id = \$2 AND user_id = \$3 ` +
				`AND lower\(user_email\) = lower\(\$4\) AND event_type IN \(\$5,\$6\) AND resource_type = \$7 ` +
				`AND resource_id = \$8 AND status = \$9 AND started_at >= \$10 AND started_at <= \$11`,
			wantArgs: 11,
		},
		{
			name:      "other than the declared types",
			filter:    Filter{OrganizationId: &orgId, OtherThan: []string{"disk.create", "disk.delete"}, Limit: 25},
			wantWhere: `WHERE organization_id = \$1 AND event_type NOT IN \(\$2,\$3\)`,
			wantArgs:  3,
		},
		{
			name: "declared types or other, next to another filter",
			filter: Filter{
				OrganizationId: &orgId, EventTypes: []string{"disk.create"}, OtherThan: []string{"disk.create", "disk.delete"},
				Status: model.AuditStatusFailed, Limit: 25,
			},
			wantWhere: `WHERE organization_id = \$1 AND \(event_type IN \(\$2\) OR event_type NOT IN \(\$3,\$4\)\) ` +
				`AND status = \$5`,
			wantArgs: 5,
		},
		{
			name:      "events without organization",
			filter:    Filter{NoOrganization: true, UserId: &userId, Limit: 25},
			wantWhere: `WHERE organization_id IS NULL AND user_id = \$1`,
			wantArgs:  1,
		},
		{
			name:      "propagates the count error",
			filter:    Filter{OrganizationId: &orgId, Limit: 25},
			wantWhere: `WHERE organization_id = \$1`,
			wantArgs:  1,
			queryErr:  errors.New("boom"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			args := make([]driver.Value, tt.wantArgs)
			for i := range args {
				args[i] = sqlmock.AnyArg()
			}

			count := mock.ExpectQuery(`SELECT count\(\*\) FROM "audit_events" ` + tt.wantWhere).
				WithArgs(args...)
			if tt.queryErr != nil {
				count.WillReturnError(tt.queryErr)
			} else {
				count.WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))
				mock.ExpectQuery(`SELECT \* FROM "audit_events" ` + tt.wantWhere +
					` ORDER BY started_at DESC, id DESC LIMIT`).
					WillReturnRows(sqlmock.NewRows([]string{"id", "event_type"}).AddRow(uuid.New(), "disk.create"))
			}

			events, total, err := List(context.Background(), tt.filter)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int64(42), total)
				assert.Len(t, events, 1)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDeleteExpired(t *testing.T) {
	orgId := uuid.New()
	cutoff := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		run     func() (int64, error)
		wantSQL string
		execErr error
		want    int64
		wantErr bool
	}{
		{
			name:    "organization batch",
			run:     func() (int64, error) { return DeleteExpiredByOrganization(context.Background(), orgId, cutoff, 100) },
			wantSQL: `DELETE FROM audit_events WHERE id IN .*organization_id = \$1 AND started_at < \$2.*LIMIT \$3`,
			want:    100,
		},
		{
			name:    "default batch skips organizations with an override",
			run:     func() (int64, error) { return DeleteExpiredDefault(context.Background(), cutoff, 100) },
			wantSQL: `DELETE FROM audit_events WHERE id IN .*organization_id IS NOT NULL.*NOT EXISTS .*audit_retention_days IS NOT NULL.*LIMIT \$2`,
			want:    100,
		},
		{
			name:    "batch without organization",
			run:     func() (int64, error) { return DeleteExpiredWithoutOrganization(context.Background(), cutoff, 100) },
			wantSQL: `DELETE FROM audit_events WHERE id IN .*organization_id IS NULL AND started_at < \$1.*LIMIT \$2`,
			want:    100,
		},
		{
			name:    "propagates the delete error",
			run:     func() (int64, error) { return DeleteExpiredDefault(context.Background(), cutoff, 100) },
			wantSQL: `DELETE FROM audit_events`,
			execErr: errors.New("boom"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, cleanup := setupMockDB(t)
			defer cleanup()

			exec := mock.ExpectExec(tt.wantSQL)
			if tt.execErr != nil {
				exec.WillReturnError(tt.execErr)
			} else {
				exec.WillReturnResult(sqlmock.NewResult(0, tt.want))
			}

			got, err := tt.run()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
