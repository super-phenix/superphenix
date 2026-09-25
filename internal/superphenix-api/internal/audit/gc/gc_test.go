package gc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// noOrg keys the events without organization in the fake store, uuid.Nil keys the default pass.
var noOrg = uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")

type fakeStore struct {
	overrides    []model.Organization
	overridesErr error
	deleteErr    error
	// remaining is how many expired events each target still holds, uuid.Nil being the default.
	remaining map[uuid.UUID]int64

	cutoffs map[uuid.UUID]time.Time
	batches int
}

func (s *fakeStore) Overrides(context.Context) ([]model.Organization, error) {
	return s.overrides, s.overridesErr
}

func (s *fakeStore) delete(target uuid.UUID, cutoff time.Time, limit int) (int64, error) {
	if s.deleteErr != nil {
		return 0, s.deleteErr
	}
	s.batches++
	s.cutoffs[target] = cutoff
	deleted := min(s.remaining[target], int64(limit))
	s.remaining[target] -= deleted
	return deleted, nil
}

func (s *fakeStore) DeleteByOrganization(_ context.Context, orgaId uuid.UUID, cutoff time.Time, limit int) (int64, error) {
	return s.delete(orgaId, cutoff, limit)
}

func (s *fakeStore) DeleteDefault(_ context.Context, cutoff time.Time, limit int) (int64, error) {
	return s.delete(uuid.Nil, cutoff, limit)
}

func (s *fakeStore) DeleteWithoutOrganization(_ context.Context, cutoff time.Time, limit int) (int64, error) {
	return s.delete(noOrg, cutoff, limit)
}

func TestSweep(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	shortOrg, longOrg := uuid.New(), uuid.New()
	days := func(n int) *int { return &n }
	organization := func(id uuid.UUID, retention int) model.Organization {
		return model.Organization{Model: model.Model{ID: id}, AuditRetentionDays: days(retention)}
	}

	tests := []struct {
		name         string
		store        *fakeStore
		lockAcquired bool
		lockErr      error
		batchSize    int
		cancelled    bool
		want         int64
		wantErr      bool
		wantBatches  int
		wantCutoffs  map[uuid.UUID]time.Time
	}{
		{
			name:         "another replica holds the lock",
			store:        &fakeStore{remaining: map[uuid.UUID]int64{uuid.Nil: 10}},
			batchSize:    100,
			lockAcquired: false,
		},
		{
			name:      "lock failure",
			store:     &fakeStore{},
			batchSize: 100,
			lockErr:   errors.New("boom"),
			wantErr:   true,
		},
		{
			name:         "default retention only",
			store:        &fakeStore{remaining: map[uuid.UUID]int64{uuid.Nil: 10}},
			lockAcquired: true,
			batchSize:    100,
			want:         10,
			wantBatches:  2,
			wantCutoffs:  map[uuid.UUID]time.Time{uuid.Nil: now.Add(-90 * day), noOrg: now.Add(-30 * day)},
		},
		{
			name:         "events without organization follow the user retention",
			store:        &fakeStore{remaining: map[uuid.UUID]int64{uuid.Nil: 2, noOrg: 3}},
			lockAcquired: true,
			batchSize:    100,
			want:         5,
			wantBatches:  2,
			wantCutoffs:  map[uuid.UUID]time.Time{uuid.Nil: now.Add(-90 * day), noOrg: now.Add(-30 * day)},
		},
		{
			name:         "batches until a short one",
			store:        &fakeStore{remaining: map[uuid.UUID]int64{uuid.Nil: 250}},
			lockAcquired: true,
			batchSize:    100,
			want:         250,
			wantBatches:  4,
			wantCutoffs:  map[uuid.UUID]time.Time{uuid.Nil: now.Add(-90 * day), noOrg: now.Add(-30 * day)},
		},
		{
			name: "overrides are clamped to the bounds",
			store: &fakeStore{
				overrides: []model.Organization{organization(shortOrg, 1), organization(longOrg, 9999)},
				remaining: map[uuid.UUID]int64{shortOrg: 5, longOrg: 7, uuid.Nil: 1},
			},
			lockAcquired: true,
			batchSize:    100,
			want:         13,
			wantBatches:  4,
			wantCutoffs: map[uuid.UUID]time.Time{
				shortOrg: now.Add(-7 * day),
				longOrg:  now.Add(-365 * day),
				uuid.Nil: now.Add(-90 * day),
				noOrg:    now.Add(-30 * day),
			},
		},
		{
			name:         "overrides listing failure",
			store:        &fakeStore{overridesErr: errors.New("boom")},
			lockAcquired: true,
			batchSize:    100,
			wantErr:      true,
		},
		{
			name:         "delete failure",
			store:        &fakeStore{deleteErr: errors.New("boom")},
			lockAcquired: true,
			batchSize:    100,
			wantErr:      true,
		},
		{
			name:         "cancelled context stops before deleting",
			store:        &fakeStore{remaining: map[uuid.UUID]int64{uuid.Nil: 10}},
			lockAcquired: true,
			batchSize:    100,
			cancelled:    true,
			wantErr:      true,
		},
		{
			name:         "non positive batch size is refused",
			store:        &fakeStore{remaining: map[uuid.UUID]int64{uuid.Nil: 10}},
			lockAcquired: true,
			batchSize:    0,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.store.cutoffs = map[uuid.UUID]time.Time{}
			released := false

			var cfg config.AuditLogConfig
			cfg.Retention.DefaultDays = 90
			cfg.Retention.MinDays = 7
			cfg.Retention.MaxDays = 365
			cfg.Retention.UserDays = 30
			cfg.GarbageCollection.BatchSize = tt.batchSize

			sweeper := &Sweeper{
				Config: cfg,
				Store:  tt.store,
				Lock: func(context.Context) (func(), bool, error) {
					return func() { released = true }, tt.lockAcquired, tt.lockErr
				},
				Now: func() time.Time { return now },
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.cancelled {
				cancel()
			}

			got, err := sweeper.Sweep(ctx)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantBatches, tt.store.batches)
			assert.Equal(t, tt.lockAcquired && tt.lockErr == nil, released)
			if tt.wantCutoffs != nil {
				assert.Equal(t, tt.wantCutoffs, tt.store.cutoffs)
			}
		})
	}
}
