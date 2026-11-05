package cluster

import (
	"strings"
	"testing"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
)

func TestValidateTopology(t *testing.T) {
	tests := []struct {
		name        string
		topology    operatorv1alpha1.DeploymentTopology
		clusterType *operatorv1alpha1.ClusterType
		wantError   string
	}{
		{
			name:        "empty topology with storage type is invalid",
			clusterType: clusterTypePtr(operatorv1alpha1.ClusterTypeStorage),
			wantError:   "cluster type must be Management when topology is empty",
		},
		{
			name:      "empty topology with missing type is invalid",
			wantError: "cluster type must be Management when topology is empty",
		},
		{
			name:        "empty topology with management type is valid",
			clusterType: clusterTypePtr(operatorv1alpha1.ClusterTypeManagement),
		},
		{
			name:        "decoupled topology with management type is invalid",
			topology:    operatorv1alpha1.DeploymentTopologyDecoupled,
			clusterType: clusterTypePtr(operatorv1alpha1.ClusterTypeManagement),
			wantError:   "cluster type must be Storage or Workload when topology is Decoupled",
		},
		{
			name:        "decoupled topology with storage type is valid",
			topology:    operatorv1alpha1.DeploymentTopologyDecoupled,
			clusterType: clusterTypePtr(operatorv1alpha1.ClusterTypeStorage),
		},
		{
			name:        "decoupled topology with workload type is valid",
			topology:    operatorv1alpha1.DeploymentTopologyDecoupled,
			clusterType: clusterTypePtr(operatorv1alpha1.ClusterTypeWorkload),
		},
		{
			name:     "hyperconverged topology is valid",
			topology: operatorv1alpha1.DeploymentTopologyHyperconverged,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&Reconciler{}).validateTopology(&operatorv1alpha1.Cluster{
				Spec: operatorv1alpha1.ClusterSpec{
					DeploymentTopology: tt.topology,
					Type:               tt.clusterType,
				},
			})

			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("validateTopology() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("validateTopology() error = %v, want an error containing %q", err, tt.wantError)
			}
		})
	}
}

func clusterTypePtr(clusterType operatorv1alpha1.ClusterType) *operatorv1alpha1.ClusterType {
	return &clusterType
}
