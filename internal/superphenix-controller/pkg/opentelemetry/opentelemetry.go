package opentelemetry

import (
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
)

type errorHandler struct{}

func (errorHandler) Handle(err error) {
	log.Error().Err(err).Msg("Telemetry error")
}

func init() {
	otel.SetErrorHandler(errorHandler{})
}
