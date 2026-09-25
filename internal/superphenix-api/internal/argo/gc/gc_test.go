package gc

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestGarbageCollectionWithoutLock(t *testing.T) {
	tests := []struct {
		name    string
		expect  func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "lock query fails",
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`pg_try_advisory_lock`).WillReturnError(errors.New("connection refused"))
			},
			wantErr: true,
		},
		{
			name: "lock held by another replica",
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`pg_try_advisory_lock`).
					WillReturnRows(sqlmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(false))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer func() { _ = sqlDB.Close() }()

			gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
			if err != nil {
				t.Fatalf("failed to open gorm: %v", err)
			}

			tt.expect(mock)
			err = garbageCollection(context.Background(), nil, gormDB)

			assert.Equal(t, tt.wantErr, err != nil)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
