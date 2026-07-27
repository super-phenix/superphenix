// Package telemetry collects anonymised usage metrics about the operator
// and the clusters it manages, and periodically pushes them to the
// Superphenix open-source telemetry endpoint.
//
// The wire schema is intentionally identical to the one accepted by
// https://github.com/super-phenix/superphenix-telemetry. Keep these
// structures and constants in sync with that project.
package telemetry

// SchemaVersion is the wire schema version the receiver expects.
const SchemaVersion = 1

// Wire-level limits mirrored from the receiving server. They exist
// primarily so that the operator never produces a payload the server
// would reject.
const (
	MaxMetricsPerReport = 50
	MaxLabelsPerMetric  = 8
	MaxLabelValueLen    = 64
	MaxBodyBytes        = 64 * 1024 // 64 KiB
)

// Allowed metric kinds.
const (
	KindCounter = "counter"
	KindGauge   = "gauge"
)

// Allowed metric names.
const (
	MetricOperatorInfo  = "operator_info"
	MetricClusterInfo   = "cluster_info"
	MetricComponentInfo = "component_info"
	MetricRegionCount   = "region_count"
	MetricAZCount       = "az_count"
	MetricNodeCount     = "node_count"
)

// Allowed values for the cluster_info "topology" label.
const (
	TopologyHyperconverged = "hyperconverged"
	TopologyDecoupled      = "decoupled"
)

// Allowed values for the cluster_info "type" label.
const (
	TypeStorage        = "storage"
	TypeVirtualization = "virtualization"
	TypeNone           = "none"
)

// Report is the top-level body posted to the ingest endpoint.
type Report struct {
	SchemaVersion  int      `json:"schema_version"`
	InstallationID string   `json:"installation_id"`
	Metrics        []Metric `json:"metrics"`
}

// Metric describes a single observation.
type Metric struct {
	Name   string            `json:"name"`
	Kind   string            `json:"kind"`
	Value  float64           `json:"value"`
	Labels map[string]string `json:"labels,omitempty"`
}
