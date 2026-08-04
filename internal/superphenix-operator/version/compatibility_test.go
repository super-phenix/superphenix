package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestIsManagementUpgradeSupported(t *testing.T) {
	oldMin := MinManagementVersionBeforeUpgrade
	MinManagementVersionBeforeUpgrade = "1.0.0"
	defer func() { MinManagementVersionBeforeUpgrade = oldMin }()

	tests := []struct {
		name    string
		current string
		target  string
		wantErr bool
	}{
		{"First install", "", "1.1.0", false},
		{"Latest target", "1.0.0", "0.0.0", false},
		{"Supported upgrade", "1.0.0", "1.1.0", false},
		{"Unsupported upgrade below min", "0.9.0", "1.1.0", true},
		{"Unsupported upgrade to baseline", "0.9.0", "1.0.0", true},
		{"Supported from baseline", "1.0.0", "1.0.0", false},
		{"Supported upgrade to newer", "1.0.0", "2.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsManagementUpgradeSupported(tt.current, tt.target)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsClusterUpgradeSupported(t *testing.T) {
	oldMin := MinClusterVersion
	oldMax := MaxClusterVersion
	MinClusterVersion = "1.0.0"
	MaxClusterVersion = "2.0.0"
	defer func() {
		MinClusterVersion = oldMin
		MaxClusterVersion = oldMax
	}()

	tests := []struct {
		name    string
		current string
		target  string
		wantErr bool
	}{
		{"Same version", "1.1.0", "1.1.0", false},
		{"Latest target", "1.1.0", "0.0.0", false},
		{"Supported upgrade", "1.0.0", "1.1.0", false},
		{"Unsupported upgrade to max", "1.1.0", "2.0.0", true},
		{"Unsupported upgrade below min", "0.9.0", "1.1.0", true},
		{"Unsupported upgrade to baseline", "0.9.0", "1.0.0", true},
		{"Unsupported upgrade above max", "1.5.0", "2.1.0", true},
		{"Unsupported upgrade from above max", "2.1.0", "2.2.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsClusterUpgradeSupported(tt.current, tt.target)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsClusterCompatibleWithManagement(t *testing.T) {
	oldMin := MinClusterVersion
	oldMax := MaxClusterVersion
	MinClusterVersion = "1.0.0"
	MaxClusterVersion = "2.0.0"
	defer func() {
		MinClusterVersion = oldMin
		MaxClusterVersion = oldMax
	}()

	tests := []struct {
		name              string
		clusterVersion    string
		managementVersion string
		wantErr           bool
	}{
		{"Supported version", "1.0.0", "1.0.0", false},
		{"Supported version newer cluster", "1.1.0", "1.0.0", false},
		{"Supported version newer mgmt", "1.0.0", "1.1.0", false},
		{"Incompatible version (at max)", "2.0.0", "1.0.0", true},
		{"Incompatible version (below min)", "0.9.0", "1.0.0", true},
		{"Incompatible version (above max)", "2.1.0", "1.0.0", true},
		{"Latest cluster incompatible", "0.0.0", "1.0.0", true},
		{"Empty management", "1.0.0", "", false},
		{"Latest management", "1.1.0", "0.0.0", false},
		{"Unknown management version (still works)", "1.0.0", "9.9.9", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsClusterCompatibleWithManagement(tt.clusterVersion, tt.managementVersion)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetCurrentManagementVersion(t *testing.T) {
	scheme := runtime.NewScheme()

	t.Run("Application exists", func(t *testing.T) {
		app := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"metadata": map[string]any{
					"name":      ManagementAppName,
					"namespace": "spx-system",
				},
				"spec": map[string]any{
					"source": map[string]any{
						"targetRevision": "1.1.0",
					},
				},
			},
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(app).Build()
		version, err := GetCurrentManagementVersion(t.Context(), c, "spx-system")
		assert.NoError(t, err)
		assert.Equal(t, "1.1.0", version)
	})

	t.Run("Application does not exist", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		version, err := GetCurrentManagementVersion(t.Context(), c, "spx-system")
		assert.NoError(t, err)
		assert.Equal(t, "", version)
	})
}

func TestGetCurrentClusterVersion(t *testing.T) {
	scheme := runtime.NewScheme()

	t.Run("Cluster application exists", func(t *testing.T) {
		app := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"metadata": map[string]any{
					"name":      "cluster1",
					"namespace": "spx-system",
				},
				"spec": map[string]any{
					"source": map[string]any{
						"targetRevision": "1.2.0",
					},
				},
			},
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(app).Build()
		version, err := GetCurrentClusterVersion(t.Context(), c, "cluster1", "spx-system")
		assert.NoError(t, err)
		assert.Equal(t, "1.2.0", version)
	})

	t.Run("Cluster application does not exist", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		version, err := GetCurrentClusterVersion(t.Context(), c, "cluster1", "spx-system")
		assert.NoError(t, err)
		assert.Equal(t, "", version)
	})
}
