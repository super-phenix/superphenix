package dbaas

import (
	"context"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGetDBaaS(t *testing.T) {
	tests := []struct {
		name         string
		objects      func(t *testing.T) []runtime.Object
		wantErr      bool
		wantNotFound bool
	}{
		{
			name: "found",
			objects: func(t *testing.T) []runtime.Object {
				return []runtime.Object{newCluster(t, testProjectId)}
			},
		},
		{
			name: "labelled for another project",
			objects: func(t *testing.T) []runtime.Object {
				return []runtime.Object{newCluster(t, testOtherProject)}
			},
			wantErr:      true,
			wantNotFound: true,
		},
		{
			name:         "missing",
			objects:      func(_ *testing.T) []runtime.Object { return nil },
			wantErr:      true,
			wantNotFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setFakeDynamicClient(t, tt.objects(t)...)
			setFakeWatcher(t, informers.SwitchLBRules, newLBRule(testNamespace(t), testLocalId, 5432, 5434))

			got, err := GetDBaaS(context.Background(), testNamespace(t), testEffectiveId(t))
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetDBaaS() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.wantNotFound && !apierrors.IsNotFound(err) {
					t.Errorf("GetDBaaS() error = %v, want NotFound", err)
				}
				return
			}
			if got.EId != testEffectiveId(t) {
				t.Errorf("EId = %q, want %q", got.EId, testEffectiveId(t))
			}
			if got.Cluster.Status.CurrentPrimary != testEffectiveId(t)+"-1" {
				t.Errorf("CurrentPrimary = %q, want %q", got.Cluster.Status.CurrentPrimary, testEffectiveId(t)+"-1")
			}
			if len(got.Endpoints) != 2 {
				t.Errorf("len(Endpoints) = %d, want 2", len(got.Endpoints))
			}
		})
	}
}
