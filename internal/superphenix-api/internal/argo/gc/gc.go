package gc

import (
	"context"
	"fmt"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/advisorylock"
	gcLog "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// sweepLockID identifies the Postgres advisory lock serialising the sweep. The
// API runs with several replicas and there is no leader election, so without it
// every replica would list and delete the same resources on every tick.
const sweepLockID int64 = 0x5350584743 // "SPXGC"

// InitGarbageCollection runs the sweep on a timer until ctx is cancelled. Only
// the replica holding the advisory lock sweeps; the others skip the tick.
func InitGarbageCollection(ctx context.Context, c *argo.Client, db *gorm.DB) {
	opts := c.Options().GC

	log.Info().Ctx(ctx).Msgf(
		"Initialize Garbage collection - running every : %s and timeout after %s",
		opts.Interval, opts.Timeout)

	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go CallWithTimeout(ctx, c, db)
		}
	}
}

// CallWithTimeout runs one sweep bounded by the configured timeout.
func CallWithTimeout(ctx context.Context, c *argo.Client, db *gorm.DB) {
	collectionCtx, cancel := context.WithTimeout(ctx, c.Options().GC.Timeout)
	defer cancel()

	processId := uuid.New().String()
	collectionCtx = context.WithValue(collectionCtx, gcLog.ProcessIdKey, fmt.Sprintf("collection-%s", processId))

	l := gcLog.GetProcessLogger(collectionCtx)
	if err := garbageCollection(collectionCtx, c, db); err != nil {
		l.Error().Ctx(collectionCtx).Err(err).Msg("Garbage collection failed.")
		return
	}
	l.Info().Ctx(collectionCtx).Msg("Operation completed successfully.")
}

func garbageCollection(ctx context.Context, c *argo.Client, db *gorm.DB) error {
	logger := gcLog.GetProcessLogger(ctx)

	release, acquired, err := advisorylock.TryLock(ctx, db, sweepLockID)
	if err != nil {
		return fmt.Errorf("acquire garbage collection lock: %w", err)
	}
	if !acquired {
		logger.Debug().Msg("Garbage collection already running on another replica, skipping")
		return nil
	}
	defer release()

	logger.Info().Msgf("Garbage collection started at : %s", time.Now().String())

	cleaners := cleanersFor(c, logger)
	appCleaner, appProjectCleaner := cleaners[0], cleaners[1]

	// Level 1 - Clean Argo Applications first
	if err := appCleaner.Clean(ctx); err != nil {
		logger.Error().Err(err).Msg(appCleaner.ErrorMessage())
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// Wait 1 min to ensure argo apps have been fully deleted
	select {
	case <-time.After(1 * time.Minute):
	case <-ctx.Done():
		return ctx.Err()
	}

	// Level 2 - Clean App Projects after applications are gone
	if err := appProjectCleaner.Clean(ctx); err != nil {
		logger.Error().Err(err).Msg(appProjectCleaner.ErrorMessage())
	}

	logger.Info().Msgf("Garbage collection finished at : %s", time.Now().String())
	return nil
}
