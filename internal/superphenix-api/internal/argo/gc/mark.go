// Package gc runs the Argo garbage collection: marking a project's resources for
// deletion on demand, and sweeping expired ones on a timer.
package gc

import (
	"context"
	"fmt"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/gc/cleaner"
	gcLog "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// cleanersFor builds the Application and AppProject cleaners for a client.
func cleanersFor(c *argo.Client, logger zerolog.Logger) []cleaner.Cleaner {
	opts := cleaner.Options{
		LabelMarkKey:        c.Options().GC.LabelMarkKey,
		Debug:               c.Options().GC.Debug,
		AppProjectNamespace: c.Options().AppProjectNamespace,
	}
	return []cleaner.Cleaner{
		cleaner.NewApp(c.Apps(), opts, logger),
		cleaner.NewAppProject(c.Apps(), opts, logger),
	}
}

// Mark stamps every Argo resource in a project namespace with a deletion
// timestamp GC.Delay in the future. Called when a project is deleted; it is
// request-scoped and idempotent, so it is not guarded by the sweep lock.
func Mark(ctx context.Context, c *argo.Client, namespace string) error {
	processId := uuid.New().String()
	ctx = context.WithValue(ctx, gcLog.ProcessIdKey, fmt.Sprintf("marking-%s-%s", namespace, processId))
	logger := gcLog.GetProcessLogger(ctx)

	deletionTimestamp := time.Now().Add(c.Options().GC.Delay)
	deletionTimestampStr := deletionTimestamp.Format(cleaner.TimestampFormat)

	logger.Info().Msgf("Labeling started at : %s", time.Now().String())

	errWg, ctxWg := errgroup.WithContext(ctx)

	for _, cl := range cleanersFor(c, logger) {
		errWg.Go(func() error {
			if err := cl.Mark(ctxWg, namespace, deletionTimestampStr); err != nil {
				logger.Error().Err(err).Msg(cl.ErrorMessage())
				return fmt.Errorf("failed to mark item")
			}
			return nil
		})
	}

	if err := errWg.Wait(); err != nil {
		logger.Err(err).Msg("Labeling failed")
		return err
	}

	logger.Info().Msgf("Labeling finished at : %s", time.Now().String())
	return nil
}
