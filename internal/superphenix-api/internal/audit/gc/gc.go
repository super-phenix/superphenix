// Package gc hard deletes the audit events past their retention.
package gc

import (
	"context"
	"fmt"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/advisorylock"
	auditEvent "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/audit-event"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/organization"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	gcLog "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// sweepLockID identifies the Postgres advisory lock serialising the sweep across replicas.
const sweepLockID int64 = 0x53505841554449 // "SPXAUDI"

const day = 24 * time.Hour

// Store is what the sweep reads and deletes.
type Store interface {
	Overrides(ctx context.Context) ([]model.Organization, error)
	DeleteByOrganization(ctx context.Context, orgaId uuid.UUID, cutoff time.Time, limit int) (int64, error)
	DeleteDefault(ctx context.Context, cutoff time.Time, limit int) (int64, error)
	DeleteWithoutOrganization(ctx context.Context, cutoff time.Time, limit int) (int64, error)
}

type dbStore struct{}

func (dbStore) Overrides(ctx context.Context) ([]model.Organization, error) {
	return organization.FindAllWithAuditRetention(ctx)
}

func (dbStore) DeleteByOrganization(ctx context.Context, orgaId uuid.UUID, cutoff time.Time, limit int) (int64, error) {
	return auditEvent.DeleteExpiredByOrganization(ctx, orgaId, cutoff, limit)
}

func (dbStore) DeleteDefault(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return auditEvent.DeleteExpiredDefault(ctx, cutoff, limit)
}

func (dbStore) DeleteWithoutOrganization(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return auditEvent.DeleteExpiredWithoutOrganization(ctx, cutoff, limit)
}

// Sweeper deletes the expired events. Lock returns acquired false when another replica sweeps.
type Sweeper struct {
	Config config.AuditLogConfig
	Store  Store
	Lock   func(ctx context.Context) (release func(), acquired bool, err error)
	Now    func() time.Time
}

// New returns the Sweeper over the database.
func New(cfg config.AuditLogConfig) *Sweeper {
	return &Sweeper{
		Config: cfg,
		Store:  dbStore{},
		Lock: func(ctx context.Context) (func(), bool, error) {
			return advisorylock.TryLock(ctx, db.Client, sweepLockID)
		},
		Now: time.Now,
	}
}

// Run sweeps on a timer until ctx is cancelled. An interrupted sweep resumes at the next tick.
func (s *Sweeper) Run(ctx context.Context) {
	opts := s.Config.GarbageCollection
	log.Info().Msgf("Initialize audit log garbage collection - running every : %s and timeout after %s",
		opts.Interval, opts.Timeout)

	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepWithTimeout(ctx)
		}
	}
}

func (s *Sweeper) sweepWithTimeout(ctx context.Context) {
	sweepCtx, cancel := context.WithTimeout(ctx, s.Config.GarbageCollection.Timeout)
	defer cancel()
	sweepCtx = context.WithValue(sweepCtx, gcLog.ProcessIdKey, fmt.Sprintf("audit-sweep-%s", uuid.NewString()))
	logger := gcLog.GetProcessLogger(sweepCtx)

	deleted, err := s.Sweep(sweepCtx)
	if err != nil {
		logger.Error().Err(err).Int64("deleted", deleted).Msg("Audit log sweep failed")
		return
	}
	logger.Info().Int64("deleted", deleted).Msg("Audit log sweep finished")
}

// Sweep runs once and returns how many events it deleted. Organizations with a retention
// override go first, then the other organizations with the default retention, then the events
// attached to no organization with the user retention.
func (s *Sweeper) Sweep(ctx context.Context) (int64, error) {
	release, acquired, err := s.Lock(ctx)
	if err != nil {
		return 0, fmt.Errorf("acquire sweep lock: %w", err)
	}
	if !acquired {
		return 0, nil
	}
	defer release()

	overrides, err := s.Store.Overrides(ctx)
	if err != nil {
		return 0, fmt.Errorf("list retention overrides: %w", err)
	}

	var total int64
	retention := s.Config.Retention
	for _, orga := range overrides {
		// Clamp to the current bounds.
		days := min(max(*orga.AuditRetentionDays, retention.MinDays), retention.MaxDays)
		deleted, err := s.deleteInBatches(ctx, func(limit int) (int64, error) {
			return s.Store.DeleteByOrganization(ctx, orga.ID, s.cutoff(days), limit)
		})
		total += deleted
		if err != nil {
			return total, fmt.Errorf("delete events of organization %s: %w", orga.ID, err)
		}
	}

	deleted, err := s.deleteInBatches(ctx, func(limit int) (int64, error) {
		return s.Store.DeleteDefault(ctx, s.cutoff(retention.DefaultDays), limit)
	})
	total += deleted
	if err != nil {
		return total, fmt.Errorf("delete events on default retention: %w", err)
	}

	deleted, err = s.deleteInBatches(ctx, func(limit int) (int64, error) {
		return s.Store.DeleteWithoutOrganization(ctx, s.cutoff(retention.UserDays), limit)
	})
	total += deleted
	if err != nil {
		return total, fmt.Errorf("delete events without organization: %w", err)
	}

	return total, nil
}

func (s *Sweeper) cutoff(days int) time.Time {
	return s.Now().Add(-time.Duration(days) * day)
}

// deleteInBatches repeats deleteBatch until a batch comes back short.
func (s *Sweeper) deleteInBatches(ctx context.Context, deleteBatch func(limit int) (int64, error)) (int64, error) {
	batchSize := s.Config.GarbageCollection.BatchSize
	if batchSize < 1 {
		return 0, fmt.Errorf("batch size must be positive, got %d", batchSize)
	}

	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}

		deleted, err := deleteBatch(batchSize)
		total += deleted
		if err != nil {
			return total, err
		}
		if deleted < int64(batchSize) {
			return total, nil
		}
	}
}
