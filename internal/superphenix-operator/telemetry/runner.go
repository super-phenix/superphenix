package telemetry

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// DefaultEndpoint is the public telemetry endpoint of the Superphenix
// open-source project.
const DefaultEndpoint = "https://telemetry.superphenix.net"

// pushInterval is the cadence at which Reports are pushed. Hardcoded by
// design to prevent abuse of the telemetry endpoint.
const pushInterval = 6 * time.Hour

// Runner is a controller-runtime manager.Runnable that periodically
// collects a Report and pushes it to the telemetry endpoint.
type Runner struct {
	Collector *Collector
	Client    *Client
}

// Start implements manager.Runnable. It blocks until ctx is cancelled.
// One push is attempted at startup so a freshly started operator
// registers immediately; subsequent pushes happen every pushInterval.
func (r *Runner) Start(ctx context.Context) error {
	log := logf.Log.WithName("telemetry")
	log.Info("Telemetry runner starting on leader", "interval", pushInterval, "endpoint", r.Client.endpoint)

	r.pushOnce(ctx, log)

	ticker := time.NewTicker(pushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.pushOnce(ctx, log)
		}
	}
}

// NeedLeaderElection ensures only the leader pushes telemetry when
// leader election is enabled, so a multi-replica deployment does not
// multiply the push rate.
func (r *Runner) NeedLeaderElection() bool {
	return true
}

func (r *Runner) pushOnce(ctx context.Context, log logr.Logger) {
	report, err := r.Collector.Collect(ctx)
	if err != nil {
		log.Error(err, "Failed to collect telemetry report")
		return
	}
	if err := r.Client.Push(ctx, report); err != nil {
		log.Error(err, "Failed to push telemetry report")
		return
	}
	log.Info("Pushed telemetry report", "metrics", len(report.Metrics))
}
