package dbaas

import (
	"reflect"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestUnstructuredToCluster(t *testing.T) {
	tests := []struct {
		name               string
		mutate             func(u *unstructured.Unstructured)
		wantInstances      int
		wantReadyInstances int
		wantPhase          string
		wantStorageSize    string
		wantConditions     int
	}{
		{
			name:               "full cluster",
			wantInstances:      2,
			wantReadyInstances: 2,
			wantPhase:          "Cluster in healthy state",
			wantStorageSize:    "10Gi",
			wantConditions:     1,
		},
		{
			name:            "no status yet",
			mutate:          func(u *unstructured.Unstructured) { unstructured.RemoveNestedField(u.Object, "status") },
			wantInstances:   2,
			wantStorageSize: "10Gi",
		},
		{
			name: "status of an unexpected type",
			mutate: func(u *unstructured.Unstructured) {
				u.Object["status"] = "broken"
			},
			wantInstances:   2,
			wantStorageSize: "10Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newCluster(t, testProjectId)
			if tt.mutate != nil {
				tt.mutate(u)
			}

			got := unstructuredToCluster(u)

			if got.Name != testEffectiveId(t) {
				t.Errorf("Name = %q, want %q", got.Name, testEffectiveId(t))
			}
			if got.Spec.Instances != tt.wantInstances {
				t.Errorf("Spec.Instances = %d, want %d", got.Spec.Instances, tt.wantInstances)
			}
			if got.Spec.Storage.Size != tt.wantStorageSize {
				t.Errorf("Spec.Storage.Size = %q, want %q", got.Spec.Storage.Size, tt.wantStorageSize)
			}
			if got.Status.Phase != tt.wantPhase {
				t.Errorf("Status.Phase = %q, want %q", got.Status.Phase, tt.wantPhase)
			}
			if got.Status.ReadyInstances != tt.wantReadyInstances {
				t.Errorf("Status.ReadyInstances = %d, want %d", got.Status.ReadyInstances, tt.wantReadyInstances)
			}
			if len(got.Status.Conditions) != tt.wantConditions {
				t.Errorf("len(Status.Conditions) = %d, want %d", len(got.Status.Conditions), tt.wantConditions)
			}
		})
	}
}

func TestUnstructuredToEndpoints(t *testing.T) {
	tests := []struct {
		name  string
		ports []int64
		want  []view.DBaaSEndpoint
	}{
		{
			name:  "single instance",
			ports: []int64{5432, 5434},
			want: []view.DBaaSEndpoint{
				{Role: "rw", Host: testVIP, Port: 5432},
				{Role: "r", Host: testVIP, Port: 5434},
			},
		},
		{
			name:  "replicas",
			ports: []int64{5432, 5433, 5434},
			want: []view.DBaaSEndpoint{
				{Role: "rw", Host: testVIP, Port: 5432},
				{Role: "ro", Host: testVIP, Port: 5433},
				{Role: "r", Host: testVIP, Port: 5434},
			},
		},
		{
			name:  "unknown port has no role",
			ports: []int64{6000},
			want:  []view.DBaaSEndpoint{{Host: testVIP, Port: 6000}},
		},
		{
			name: "no ports",
			want: []view.DBaaSEndpoint{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unstructuredToEndpoints(newLBRule("ns", testLocalId, tt.ports...))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unstructuredToEndpoints() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
