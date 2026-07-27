package main

import (
	"context"
	"errors"
	l "log"
	"os"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/gc"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/metrics"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/opentelemetry/tracing"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defer cleanup(ctx)

	loadConfig()
	startLogging()
	startTracing()
	startMetrics()

	spxId.SetFrameworkPrefix(config.Global.SpxPrefix)

	config.InitK8SConfig()

	go gc.InitGarbageCollection(ctx)
	api.LaunchEndpoint(config.Global.Http.Address)
}

func loadConfig() {
	if err := config.LoadConfig(); err != nil {
		if errors.Is(err, config.FileNotFound) {
			l.Print("Config file not found, falling back to environment variables and defaults")
		} else {
			l.Fatalf("An error occured while loading config file: %v", err)
		}
	}
}

func startLogging() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if config.Global.Logging.Pretty {
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).With().Timestamp().Caller().Logger()
	} else {
		log.Logger = log.Logger.With().Timestamp().Caller().Logger()
	}

	log.Debug().Msg("Starting logging")
}

func startTracing() {
	if !config.Global.Tracing.Enabled {
		return
	}

	err := tracing.StartTracing()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start tracer")
	}

	log.Debug().Msg("Starting tracing")
}

func startMetrics() {
	if !config.Global.Metrics.Enabled {
		return
	}

	log.Debug().Msg("Starting collecting and exposing metrics")

	go func() {
		err := metrics.StartMetrics()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to start metric endpoint")
		}
	}()
}

// Post shutdown cleanup
func cleanup(ctx context.Context) {
	tracing.StopTracing(ctx)
}
