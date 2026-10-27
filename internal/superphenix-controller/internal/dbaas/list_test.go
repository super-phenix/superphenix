package dbaas

import (
	"context"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestListDBaaS(t *testing.T) {
	tests := []struct {
		name          string
		noWatcher     bool
		clusters      func(t *testing.T) []*unstructured.Unstructured
		lbRules       func(t *testing.T) []*unstructured.Unstructured
		wantErr       bool
		wantCount     int
		wantEndpoints int
	}{
		{
			name: "cluster with its load balancer",
			clusters: func(t *testing.T) []*unstructured.Unstructured {
				return []*unstructured.Unstructured{newCluster(t, testProjectId)}
			},
			lbRules: func(t *testing.T) []*unstructured.Unstructured {
				return []*unstructured.Unstructured{newLBRule(testNamespace(t), testLocalId, 5432, 5434)}
			},
			wantCount:     1,
			wantEndpoints: 2,
		},
		{
			name: "load balancer of another project is ignored",
			clusters: func(t *testing.T) []*unstructured.Unstructured {
				return []*unstructured.Unstructured{newCluster(t, testProjectId)}
			},
			lbRules: func(t *testing.T) []*unstructured.Unstructured {
				return []*unstructured.Unstructured{newLBRule("spx-"+testOtherProject, testLocalId, 5432)}
			},
			wantCount: 1,
		},
		{
			name: "load balancer of another database is ignored",
			clusters: func(t *testing.T) []*unstructured.Unstructured {
				return []*unstructured.Unstructured{newCluster(t, testProjectId)}
			},
			lbRules: func(t *testing.T) []*unstructured.Unstructured {
				return []*unstructured.Unstructured{newLBRule(testNamespace(t), testOtherLocalId, 5432)}
			},
			wantCount: 1,
		},
		{
			name: "cluster labelled for another project is filtered",
			clusters: func(t *testing.T) []*unstructured.Unstructured {
				return []*unstructured.Unstructured{newCluster(t, testOtherProject)}
			},
		},
		{
			name: "empty namespace",
		},
		{
			name:      "informer not initialized",
			noWatcher: true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.noWatcher {
				clearWatcher(t, informers.PostgresCluster)
			} else {
				var clusters []*unstructured.Unstructured
				if tt.clusters != nil {
					clusters = tt.clusters(t)
				}
				setFakeWatcher(t, informers.PostgresCluster, clusters...)
			}
			var lbRules []*unstructured.Unstructured
			if tt.lbRules != nil {
				lbRules = tt.lbRules(t)
			}
			setFakeWatcher(t, informers.SwitchLBRules, lbRules...)

			got, err := ListDBaaS(context.Background(), testNamespace(t))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ListDBaaS() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if len(got) != tt.wantCount {
				t.Fatalf("len(ListDBaaS()) = %d, want %d", len(got), tt.wantCount)
			}
			if tt.wantCount == 0 {
				return
			}
			if got[0].ID != testLocalId {
				t.Errorf("ID = %q, want %q", got[0].ID, testLocalId)
			}
			if got[0].ProductName != "orders" {
				t.Errorf("ProductName = %q, want %q", got[0].ProductName, "orders")
			}
			if len(got[0].Endpoints) != tt.wantEndpoints {
				t.Errorf("len(Endpoints) = %d, want %d", len(got[0].Endpoints), tt.wantEndpoints)
			}
		})
	}
}
