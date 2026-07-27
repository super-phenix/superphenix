package alerting

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/utils"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const Namespace = "spx_gc"

const (
	ProcessLabeling = "labeling"
	ProcessCleaning = "cleaning"

	ErrorListing  = "listing"
	ErrorLabeling = "labeling"
	ErrorCleaning = "cleaning"
	ErrorParsing  = "parsing"
	ErrorTimeout  = "timeout"
)

var (
	GCErrorAlert = promauto.NewCounterVec(prometheus.CounterOpts{
		Name:      "alert",
		Namespace: Namespace,
	}, []string{
		"process_id",
		"process_type", // labeling || cleaning
		"error_type",   // listing || labeling || cleaning
		"resource_type",
		"namespace",
	})
)

func RaiseAlert(ctx context.Context, processType, errorType, resourceType string, resource utils.Resource) {
	processId := ctx.Value(logger.ProcessIdKey)
	if processId == nil {
		processId = "default"
	}

	namespace := ""
	if resource != nil {
		namespace = resource.GetLabels()[spxId.SpxLabelProjectID] // We don't use the namespace to cover non namespaced resources
	}

	GCErrorAlert.WithLabelValues(
		processId.(string),
		processType,
		errorType,
		resourceType,
		namespace,
	).Inc()
}
