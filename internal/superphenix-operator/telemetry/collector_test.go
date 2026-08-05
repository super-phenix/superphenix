package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/super-phenix/superphenix-telemetry/pkg/anonymizer"
	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var testAnon, _ = anonymizer.New("superphenix-telemetry-salt")

func testAnonymize(s string) string {
	return testAnon.Hash(s)
}

func TestCollector_Collect(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = operatorv1alpha1.SchemeBuilder.AddToScheme(scheme)

	cluster1 := &operatorv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-1", UID: "uid-1"},
		Spec: operatorv1alpha1.ClusterSpec{
			Region:             "us-east-1",
			AvailabilityZone:   "us-east-1a",
			DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
		},
		Status: operatorv1alpha1.ClusterStatus{
			SuperphenixVersion: "v1.2.3",
			NodeCount:          3,
		},
	}
	cluster2 := &operatorv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-2", UID: "uid-2"},
		Spec: operatorv1alpha1.ClusterSpec{
			Region:             "us-east-1",
			AvailabilityZone:   "us-east-1b",
			DeploymentTopology: operatorv1alpha1.DeploymentTopologyDecoupled,
			Type:               ptr(operatorv1alpha1.ClusterTypeStorage),
		},
		Status: operatorv1alpha1.ClusterStatus{
			SuperphenixVersion: "v1.2.4",
			NodeCount:          5,
		},
	}
	cluster3 := &operatorv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-3", UID: "uid-3"},
		Spec: operatorv1alpha1.ClusterSpec{
			Region:             "eu-west-1",
			AvailabilityZone:   "eu-west-1a",
			DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
		},
		Status: operatorv1alpha1.ClusterStatus{
			SuperphenixVersion: "v1.2.3",
		},
	}
	cluster4 := &operatorv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-4", UID: "uid-4"},
		Spec: operatorv1alpha1.ClusterSpec{
			Region:             "us-east-1",
			AvailabilityZone:   "us-east-1b", // Same as cluster 2
			DeploymentTopology: operatorv1alpha1.DeploymentTopologyHyperconverged,
		},
	}
	cluster5 := &operatorv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-5", UID: "uid-5"},
		Spec: operatorv1alpha1.ClusterSpec{
			Region:             "us-east-1",
			AvailabilityZone:   "us-east-1c",
			DeploymentTopology: "",
			Type:               ptr(operatorv1alpha1.ClusterTypeManagement),
		},
	}

	kubeSystem := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-system", UID: "kube-system-uid"},
	}

	c := &Collector{
		Client:          fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(cluster1, cluster2, cluster3, cluster4, cluster5, kubeSystem).Build(),
		OperatorVersion: "v1.0.0",
		Namespace:       "operator-ns",
		SystemVersion:   "v2.0.0",
		ArgoCDVersion:   "v9.7.0",
	}

	report, err := c.Collect(context.Background())
	require.NoError(t, err)

	assert.Equal(t, SchemaVersion, report.SchemaVersion)
	assert.Equal(t, testAnonymize("kube-system-uid"), report.InstallationID)

	// Check operator_info
	found := false
	for _, m := range report.Metrics {
		if m.Name == MetricOperatorInfo {
			assert.Equal(t, "v1.0.0", m.Labels["version"])
			found = true
		}
	}
	assert.True(t, found, "operator_info missing")

	// Check region_count
	found = false
	for _, m := range report.Metrics {
		if m.Name == MetricRegionCount {
			assert.Equal(t, float64(2), m.Value)
			found = true
		}
	}
	assert.True(t, found, "region_count missing")

	// Check az_count
	foundAZs := make(map[string]map[string]float64)
	for _, m := range report.Metrics {
		if m.Name == MetricAZCount {
			r := m.Labels["region"]
			if foundAZs[r] == nil {
				foundAZs[r] = make(map[string]float64)
			}
			foundAZs[r][m.Labels["az"]] = m.Value
		}
	}
	assert.Equal(t, 2, len(foundAZs))
	assert.Equal(t, 3, len(foundAZs[testAnonymize("us-east-1")]))
	assert.Equal(t, float64(1), foundAZs[testAnonymize("us-east-1")][testAnonymize("us-east-1a")])
	assert.Equal(t, float64(1), foundAZs[testAnonymize("us-east-1")][testAnonymize("us-east-1b")])
	assert.Equal(t, float64(1), foundAZs[testAnonymize("us-east-1")][testAnonymize("us-east-1c")])
	assert.Equal(t, 1, len(foundAZs[testAnonymize("eu-west-1")]))
	assert.Equal(t, float64(1), foundAZs[testAnonymize("eu-west-1")][testAnonymize("eu-west-1a")])

	// Check cluster_info
	clustersFound := 0
	for _, m := range report.Metrics {
		if m.Name == MetricClusterInfo {
			clustersFound++
			switch m.Labels["cluster"] {
			case testAnonymize("uid-1"):
				assert.Equal(t, testAnonymize("us-east-1"), m.Labels["region"])
				assert.Equal(t, testAnonymize("us-east-1a"), m.Labels["az"])
				assert.Equal(t, "hyperconverged", m.Labels["topology"])
				assert.Equal(t, "none", m.Labels["type"])
				assert.Equal(t, "v1.2.3", m.Labels["version"])
			case testAnonymize("uid-2"):
				assert.Equal(t, testAnonymize("us-east-1"), m.Labels["region"])
				assert.Equal(t, testAnonymize("us-east-1b"), m.Labels["az"])
				assert.Equal(t, "decoupled", m.Labels["topology"])
				assert.Equal(t, "storage", m.Labels["type"])
				assert.Equal(t, "v1.2.4", m.Labels["version"])
			case testAnonymize("uid-3"):
				assert.Equal(t, testAnonymize("eu-west-1"), m.Labels["region"])
				assert.Equal(t, testAnonymize("eu-west-1a"), m.Labels["az"])
			case testAnonymize("uid-4"):
				assert.Equal(t, testAnonymize("us-east-1"), m.Labels["region"])
				assert.Equal(t, testAnonymize("us-east-1b"), m.Labels["az"])
			case testAnonymize("uid-5"):
				assert.Equal(t, "hyperconverged", m.Labels["topology"])
				assert.Equal(t, "management", m.Labels["type"])
			}
		}
	}
	assert.Equal(t, 5, clustersFound)

	// Check node_count
	nodeCountsFound := 0
	for _, m := range report.Metrics {
		if m.Name == MetricNodeCount {
			nodeCountsFound++
			switch m.Labels["cluster"] {
			case testAnonymize("uid-1"):
				assert.Equal(t, float64(3), m.Value)
			case testAnonymize("uid-2"):
				assert.Equal(t, float64(5), m.Value)
			default:
				t.Errorf("unexpected node_count for cluster %s", m.Labels["cluster"])
			}
		}
	}
	assert.Equal(t, 2, nodeCountsFound)

	// Check component_info for superphenix-system
	systemComponentsFound := 0
	for _, m := range report.Metrics {
		if m.Name == MetricComponentInfo && m.Labels["name"] == "superphenix-system" && m.Labels["management"] != "true" {
			systemComponentsFound++
			switch m.Labels["cluster"] {
			case testAnonymize("uid-1"):
				assert.Equal(t, "v1.2.3", m.Labels["version"])
			case testAnonymize("uid-2"):
				assert.Equal(t, "v1.2.4", m.Labels["version"])
			case testAnonymize("uid-3"):
				assert.Equal(t, "v1.2.3", m.Labels["version"])
			case testAnonymize("uid-4"):
				assert.Equal(t, "unknown", m.Labels["version"])
			case testAnonymize("uid-5"):
				assert.Equal(t, "unknown", m.Labels["version"])
			default:
				t.Errorf("unexpected superphenix-system for cluster %s", m.Labels["cluster"])
			}
		}
	}
	assert.Equal(t, 5, systemComponentsFound)

	// Check management component info
	mgmtFound := false
	for _, m := range report.Metrics {
		if m.Name == MetricComponentInfo && m.Labels["name"] == "management" && m.Labels["management"] == "true" {
			mgmtFound = true
			assert.Equal(t, "v2.0.0", m.Labels["version"])
			_, clusterPresent := m.Labels["cluster"]
			assert.False(t, clusterPresent, "cluster label should not be present for management system info")
		}
	}
	assert.True(t, mgmtFound, "management system info missing")

	// Check argocd component info
	argocdFound := false
	for _, m := range report.Metrics {
		if m.Name == MetricComponentInfo && m.Labels["name"] == "argocd" {
			argocdFound = true
			assert.Equal(t, "v9.7.0", m.Labels["version"])
			_, clusterPresent := m.Labels["cluster"]
			assert.False(t, clusterPresent, "cluster label should not be present for argocd")
		}
	}
	assert.True(t, argocdFound, "argocd component info missing")
}

func ptr[T any](v T) *T {
	return &v
}
