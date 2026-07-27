package metrics

import (
	"net/http"
	"strconv"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

const (
	Namespace = config.AppName
)

// StartMetrics starts the metric endpoint
func StartMetrics() error {
	endpoint := config.Global.Metrics.Endpoint
	address := config.Global.Metrics.Address
	port := config.Global.Metrics.Port

	log.Debug().
		Str("endpoint", endpoint).
		Str("address", address).
		Int("port", port).
		Msg("Starting Prometheus endpoint")

	http.Handle(endpoint, promhttp.Handler())
	return http.ListenAndServe(address+":"+strconv.Itoa(port), nil)
}
