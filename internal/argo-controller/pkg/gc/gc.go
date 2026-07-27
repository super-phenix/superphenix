package gc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/super-phenix/superphenix/internal/argo-controller/internal/gc/resources/appproject"
	"github.com/super-phenix/superphenix/internal/argo-controller/internal/gc/resources/argoapp"
	"github.com/super-phenix/superphenix/internal/argo-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/config"
	gcLog "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func InitGarbageCollection(ctx context.Context) {
	log.Info().Ctx(ctx).Msgf(
		"Initialize Garbage collection - running every : %s and timeout after %s",
		config.Global.GarbageCollection.Interval,
		config.Global.GarbageCollection.Timeout)

	for range time.Tick(config.Global.GarbageCollection.Interval) {
		go CallWithTimeout(ctx, garbageCollection)
	}
}

func CallWithTimeout(ctx context.Context, call func(context.Context, chan<- bool)) {
	collectionCtx, cancel := context.WithTimeout(ctx, config.Global.GarbageCollection.Timeout)
	defer cancel()

	processId := uuid.New().String()
	collectionCtx = context.WithValue(collectionCtx, gcLog.ProcessIdKey, fmt.Sprintf("collection-%s", processId))

	processDone := make(chan bool, 1)

	go call(collectionCtx, processDone)
	l := gcLog.GetProcessLogger(collectionCtx)
	select {
	case <-processDone:
		l.Info().Ctx(collectionCtx).Msgf("Operation completed successfully.")
		return
	case <-collectionCtx.Done():
		l.Error().Ctx(collectionCtx).Msgf("Operation canceled or timed out.")
		return
	}
}

func garbageCollection(ctx context.Context, processDone chan<- bool) {
	logger := gcLog.GetProcessLogger(ctx)

	logger.Info().Msgf("Garbage collection started at : %s", time.Now().String())

	// Level 1 - Clean Argo Applications first
	var wgLvl1 sync.WaitGroup

	cleanersLvl1 := []utils.Cleaner{
		&argoapp.Cleaner{ResourceType: "Argo Application", Logger: logger},
	}

	for _, cleaner := range cleanersLvl1 {
		wgLvl1.Go(func() {
			if err := cleaner.Clean(ctx); err != nil {
				logger.Error().Err(err).Msg(cleaner.ErrorMessage())
			}
		})
	}

	wgLvl1.Wait()

	if ctx.Err() != nil {
		return
	}

	// Wait 1 min to ensure argo apps have been fully deleted
	time.Sleep(1 * time.Minute)

	// Level 2 - Clean App Projects after applications are gone
	cleanersLvl2 := []utils.Cleaner{
		&appproject.Cleaner{ResourceType: "App Project", Logger: logger},
	}

	for _, cleaner := range cleanersLvl2 {
		if err := cleaner.Clean(ctx); err != nil {
			logger.Error().Err(err).Msg(cleaner.ErrorMessage())
		}
	}

	logger.Info().Msgf("Garbage collection finished at : %s", time.Now().String())
	processDone <- true
}
