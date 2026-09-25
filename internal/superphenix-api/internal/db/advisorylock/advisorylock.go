// Package advisorylock serialises background work across the API replicas with Postgres
// advisory locks. A periodic job takes its lock on every tick and skips the tick when another
// replica holds it.
package advisorylock

import (
	"context"
	"database/sql/driver"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// releaseTimeout bounds the unlock, which runs on a fresh context.
const releaseTimeout = 5 * time.Second

// TryLock takes the advisory lock on a dedicated connection and releases it on that same
// connection. acquired is false when another replica holds the lock.
func TryLock(ctx context.Context, db *gorm.DB, id int64) (release func(), acquired bool, err error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, false, err
	}

	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, false, err
	}

	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", id).Scan(&acquired); err != nil {
		_ = conn.Close()
		return nil, false, err
	}

	if !acquired {
		_ = conn.Close()
		return nil, false, nil
	}

	release = func() {
		// The caller's context may already be cancelled or timed out.
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
		defer cancel()
		if _, err := conn.ExecContext(unlockCtx, "SELECT pg_advisory_unlock($1)", id); err != nil {
			log.Error().Err(err).Int64("lockId", id).Msg("Failed to release advisory lock")
			// Discard the connection so its session, and the lock with it, ends.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = conn.Close()
	}

	return release, true, nil
}
