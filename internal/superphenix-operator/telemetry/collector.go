package telemetry

import (
	"context"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/super-phenix/superphenix-telemetry/pkg/anonymizer"
	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
)

// labelValueRe mirrors the upstream label value pattern. Values that
// would be rejected by the server are dropped at collection time so the
// rest of the report can still go through.
var labelValueRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// Collector builds a Report from the operator's view of the world by
// listing Cluster CRs from the local cache. It never contacts remote
// clusters.
type Collector struct {
	Client          client.Reader
	OperatorVersion string
	Namespace       string

	// SystemVersion is the currently deployed management
	// chart version on this cluster (in management mode).
	SystemVersion string

	// ArgoCDVersion is the currently deployed argocd chart version on
	// this cluster.
	ArgoCDVersion string
}

// Collect returns the current Report. It always includes operator_info;
// AZ-related metrics are included only when at least one Cluster CR is
// readable.
func (c *Collector) Collect(ctx context.Context) (Report, error) {
	report := Report{SchemaVersion: SchemaVersion}

	instUID := c.getInstallationUID(ctx)
	// We use a fixed salt for the anonymizer to remain consistent across restarts.
	// UIDs are already unique, so a fixed salt is sufficient to make them opaque.
	anon, _ := anonymizer.New("superphenix-telemetry-salt")
	report.InstallationID = anon.Hash(instUID)

	report.Metrics = append(report.Metrics, Metric{
		Name:   MetricOperatorInfo,
		Kind:   KindGauge,
		Value:  1,
		Labels: map[string]string{"version": sanitizeVersion(c.OperatorVersion)},
	})

	if c.SystemVersion != "" {
		report.Metrics = append(report.Metrics, Metric{
			Name:  MetricComponentInfo,
			Kind:  KindGauge,
			Value: 1,
			Labels: map[string]string{
				"name":       "management",
				"version":    sanitizeVersion(c.SystemVersion),
				"management": "true",
			},
		})
	}

	if c.ArgoCDVersion != "" {
		report.Metrics = append(report.Metrics, Metric{
			Name:  MetricComponentInfo,
			Kind:  KindGauge,
			Value: 1,
			Labels: map[string]string{
				"name":    "argocd",
				"version": sanitizeVersion(c.ArgoCDVersion),
			},
		})
	}

	clusters := &operatorv1alpha1.ClusterList{}
	if err := c.Client.List(ctx, clusters); err != nil {
		return report, err
	}

	regionAZs := make(map[string]map[string]struct{})
	for i := range clusters.Items {
		r := clusters.Items[i].Spec.Region
		az := clusters.Items[i].Spec.AvailabilityZone
		if regionAZs[r] == nil {
			regionAZs[r] = make(map[string]struct{})
		}
		regionAZs[r][az] = struct{}{}
	}

	report.Metrics = append(report.Metrics, Metric{
		Name:  MetricRegionCount,
		Kind:  KindGauge,
		Value: float64(len(regionAZs)),
	})

	for region, azs := range regionAZs {
		for az := range azs {
			report.Metrics = append(report.Metrics, Metric{
				Name:  MetricAZCount,
				Kind:  KindGauge,
				Value: 1,
				Labels: map[string]string{
					"region": anon.Hash(region),
					"az":     anon.Hash(az),
				},
			})
		}
	}

	for i := range clusters.Items {
		cl := &clusters.Items[i]
		report.Metrics = append(report.Metrics, Metric{
			Name:  MetricClusterInfo,
			Kind:  KindGauge,
			Value: 1,
			Labels: map[string]string{
				"cluster":  anon.Hash(string(cl.UID)),
				"region":   anon.Hash(cl.Spec.Region),
				"az":       anon.Hash(cl.Spec.AvailabilityZone),
				"topology": topologyLabel(cl.Spec.DeploymentTopology),
				"type":     typeLabel(cl.Spec.DeploymentTopology, cl.Spec.Type),
				"version":  sanitizeVersion(cl.Status.SuperphenixVersion),
			},
		})

		report.Metrics = append(report.Metrics, Metric{
			Name:  MetricComponentInfo,
			Kind:  KindGauge,
			Value: 1,
			Labels: map[string]string{
				"cluster": anon.Hash(string(cl.UID)),
				"name":    "superphenix-system",
				"version": sanitizeVersion(cl.Status.SuperphenixVersion),
			},
		})

		if cl.Status.NodeCount > 0 {
			report.Metrics = append(report.Metrics, Metric{
				Name:  MetricNodeCount,
				Kind:  KindGauge,
				Value: float64(cl.Status.NodeCount),
				Labels: map[string]string{
					"cluster": anon.Hash(string(cl.UID)),
				},
			})
		}
	}

	if len(report.Metrics) > MaxMetricsPerReport {
		report.Metrics = report.Metrics[:MaxMetricsPerReport]
	}

	return report, nil
}

// getInstallationUID fetches the UID of the kube-system namespace or fallbacks to the operator namespace.
func (c *Collector) getInstallationUID(ctx context.Context) string {
	ns := &corev1.Namespace{}
	if err := c.Client.Get(ctx, client.ObjectKey{Name: "kube-system"}, ns); err == nil {
		return string(ns.UID)
	}
	if c.Namespace != "" {
		if err := c.Client.Get(ctx, client.ObjectKey{Name: c.Namespace}, ns); err == nil {
			return string(ns.UID)
		}
	}
	return "unknown"
}

func topologyLabel(t operatorv1alpha1.DeploymentTopology) string {
	switch t {
	case operatorv1alpha1.DeploymentTopologyHyperconverged:
		return TopologyHyperconverged
	case operatorv1alpha1.DeploymentTopologyDecoupled:
		return TopologyDecoupled
	default:
		return TopologyHyperconverged
	}
}

func typeLabel(topo operatorv1alpha1.DeploymentTopology, t *operatorv1alpha1.ClusterType) string {
	if t != nil && *t == operatorv1alpha1.ClusterTypeManagement {
		return TypeManagement
	}
	if topo == operatorv1alpha1.DeploymentTopologyHyperconverged || t == nil {
		return TypeNone
	}
	switch *t {
	case operatorv1alpha1.ClusterTypeStorage:
		return TypeStorage
	case operatorv1alpha1.ClusterTypeWorkload:
		return TypeWorkload
	default:
		return TypeNone
	}
}

// sanitizeVersion coerces a version string into something the server's
// label value regex will accept. Empty or invalid inputs collapse to
// "unknown" so the report still validates.
func sanitizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "unknown"
	}
	if len(v) > MaxLabelValueLen {
		v = v[:MaxLabelValueLen]
	}
	if !labelValueRe.MatchString(v) {
		return "unknown"
	}
	return v
}
