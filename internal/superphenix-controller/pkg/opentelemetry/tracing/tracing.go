package tracing

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

var (
	traceProvider *tracesdk.TracerProvider
)

// StartTracing starts a new tracing provider
func StartTracing() error {
	if err := setTracerProvider(config.Global.Tracing.Address); err != nil {
		return err
	}

	// Register our TracerProvider as the global so any imported instrumentation in the future will default to using it
	otel.SetTracerProvider(traceProvider)
	return nil
}

// StopTracing closes the tracing provider
func StopTracing(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, config.Global.Tracing.CleanupTimeout)
	defer cancel()
	if err := traceProvider.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to properly shutdown the trace provider")
	}
}

// setTracerProvider sets an OpenTelemetry TracerProvider configured to use
// the Jaeger exporter that will send spans to the provided url. The set
// TracerProvider will also use a Resource configured with all the information
// about the application.
func setTracerProvider(url string) error {
	// Create the Jaeger exporter
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return err
	}

	traceProvider = tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exporter, tracesdk.WithBatchTimeout(config.Global.Tracing.BatchTimeout)),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.AppName),
		)),
	)

	return nil
}
