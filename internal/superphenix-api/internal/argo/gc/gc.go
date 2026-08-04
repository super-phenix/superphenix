package gc

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo"
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

	processDone := make(chan bool, 1)

	go garbageCollection(collectionCtx, c, db, processDone)
	l := gcLog.GetProcessLogger(collectionCtx)
	select {
	case <-processDone:
		l.Info().Ctx(collectionCtx).Msg("Operation completed successfully.")
	case <-collectionCtx.Done():
		l.Error().Ctx(collectionCtx).Msg("Operation canceled or timed out.")
	}
}

func garbageCollection(ctx context.Context, c *argo.Client, db *gorm.DB, processDone chan<- bool) {
	logger := gcLog.GetProcessLogger(ctx)

	conn, release, err := acquireSweepLock(ctx, db)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to acquire garbage collection lock")
		return
	}
	if conn == nil {
		logger.Debug().Msg("Garbage collection already running on another replica, skipping")
		processDone <- true
		return
	}
	defer release()

	logger.Info().Msgf("Garbage collection started at : %s", time.Now().String())

	cleaners := cleanersFor(c, logger)
	appCleaner, appProjectCleaner := cleaners[0], cleaners[1]

	// Level 1 - Clean Argo Applications first
	if err := appCleaner.Clean(ctx); err != nil {
		logger.Error().Err(err).Msg(appCleaner.ErrorMessage())
	}

	if ctx.Err() != nil {
		return
	}

	// Wait 1 min to ensure argo apps have been fully deleted
	select {
	case <-time.After(1 * time.Minute):
	case <-ctx.Done():
		return
	}

	// Level 2 - Clean App Projects after applications are gone
	if err := appProjectCleaner.Clean(ctx); err != nil {
		logger.Error().Err(err).Msg(appProjectCleaner.ErrorMessage())
	}

	logger.Info().Msgf("Garbage collection finished at : %s", time.Now().String())
	processDone <- true
}

// acquireSweepLock takes the advisory lock on a dedicated connection — the lock
// is session-scoped, so it must be released on the same connection it was taken.
// A nil conn with a nil error means another replica holds it. If this replica
// crashes, its session ends and Postgres releases the lock on its own.
func acquireSweepLock(ctx context.Context, db *gorm.DB) (*sql.Conn, func(), error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}

	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}

	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", sweepLockID).Scan(&acquired); err != nil {
		conn.Close()
		return nil, nil, err
	}

	if !acquired {
		conn.Close()
		return nil, nil, nil
	}

	release := func() {
		// Use a fresh context: the sweep's may already be cancelled or timed out.
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(unlockCtx, "SELECT pg_advisory_unlock($1)", sweepLockID); err != nil {
			log.Error().Err(err).Msg("Failed to release garbage collection lock")
		}
		conn.Close()
	}

	return conn, release, nil
}
