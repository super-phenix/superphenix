package kaas

import (
	"context"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
)

func TestCreateKaaSAppValues_ClusterFriendlyName(t *testing.T) {
	config.Global.ProductsConfig.ArgoApp.Kubernetes.KubeVersions = []config.KubeVersionConfig{{Version: "1.35.0"}}
	ctx := context.Background()
	kaasConfig := KaaSConfig{
		StorageClasses: []ClassMapping{{Shortname: "sc1", Fullname: "storage-class-1"}},
	}

	newSpec := func(essentials EssentialsSpec) KaaSSpec {
		return KaaSSpec{
			KubeVersion:   "1.35.0",
			CPNetPol:      "default",
			WorkersNetPol: "default",
			Groups: []Group{{
				Name:         "group-1",
				Replicas:     3,
				Cpu:          2,
				Memory:       4,
				BootDiskSize: 20,
				StorageClass: "sc1",
				Subnets:      []GroupSubnet{{Order: 1, Id: "subnet-1"}},
			}},
			KaasEssentials: essentials,
		}
	}
	oldSpec := func(essentials EssentialsSpec) *KaaSSpec {
		spec := newSpec(essentials)
		return &spec
	}

	tests := []struct {
		name              string
		spec              KaaSSpec
		oldSpec           *KaaSSpec
		expectedStrings   []string
		unexpectedStrings []string
	}{
		{
			name:            "create with name",
			spec:            newSpec(EssentialsSpec{ClusterFriendlyName: "my-cluster"}),
			expectedStrings: []string{"clusterFriendlyName: my-cluster", "revision: 1"},
		},
		{
			name:              "create without name",
			spec:              newSpec(EssentialsSpec{}),
			expectedStrings:   []string{"revision: 1"},
			unexpectedStrings: []string{"clusterFriendlyName"},
		},
		{
			name:              "update with same name",
			spec:              newSpec(EssentialsSpec{ClusterFriendlyName: "my-cluster"}),
			oldSpec:           oldSpec(EssentialsSpec{ClusterFriendlyName: "my-cluster", Revision: 1}),
			expectedStrings:   []string{"clusterFriendlyName: my-cluster", "revision: 1"},
			unexpectedStrings: []string{"revision: 2"},
		},
		{
			name:            "update with changed name",
			spec:            newSpec(EssentialsSpec{ClusterFriendlyName: "renamed"}),
			oldSpec:         oldSpec(EssentialsSpec{ClusterFriendlyName: "my-cluster", Revision: 3}),
			expectedStrings: []string{"clusterFriendlyName: renamed", "revision: 4"},
		},
		{
			name:            "update of a cluster without name",
			spec:            newSpec(EssentialsSpec{ClusterFriendlyName: "my-cluster"}),
			oldSpec:         oldSpec(EssentialsSpec{Revision: 2}),
			expectedStrings: []string{"clusterFriendlyName: my-cluster", "revision: 3"},
		},
		{
			name: "update with changed name and essentials values",
			spec: newSpec(EssentialsSpec{ClusterFriendlyName: "renamed", CorednsValues: "replicaCount: 3"}),
			oldSpec: oldSpec(EssentialsSpec{
				ClusterFriendlyName: "my-cluster",
				CorednsValues:       "replicaCount: 2",
				Revision:            3,
			}),
			expectedStrings: []string{"clusterFriendlyName: renamed", "revision: 4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, _, err := CreateKaaSAppValues(ctx, "test-cluster", "test-loc", tt.spec, kaasConfig, tt.oldSpec)
			if err != nil {
				t.Fatalf("CreateKaaSAppValues() error = %v", err)
			}
			for _, expected := range tt.expectedStrings {
				if !strings.Contains(values, expected) {
					t.Errorf("values do not contain %q\n%s", expected, values)
				}
			}
			for _, unexpected := range tt.unexpectedStrings {
				if strings.Contains(values, unexpected) {
					t.Errorf("values contain %q\n%s", unexpected, values)
				}
			}

			got, err := ConvertAppToUpdateKaaSSpec(appViewWithHelmValues(values))
			if err != nil {
				t.Fatalf("ConvertAppToUpdateKaaSSpec() error = %v", err)
			}
			if got.KaasEssentials.ClusterFriendlyName != tt.spec.KaasEssentials.ClusterFriendlyName {
				t.Errorf("ClusterFriendlyName = %q, want %q",
					got.KaasEssentials.ClusterFriendlyName, tt.spec.KaasEssentials.ClusterFriendlyName)
			}
		})
	}
}
