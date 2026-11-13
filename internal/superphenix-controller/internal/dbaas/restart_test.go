package dbaas

import (
	"context"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestRestart(t *testing.T) {
	tests := []struct {
		name         string
		objects      func(t *testing.T) []runtime.Object
		wantErr      bool
		wantNotFound bool
	}{
		{
			name: "annotation set",
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
			client := setFakeDynamicClient(t, tt.objects(t)...)
			namespace := testNamespace(t)
			eid := testEffectiveId(t)

			err := Restart(context.Background(), namespace, eid)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Restart() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.wantNotFound && !apierrors.IsNotFound(err) {
					t.Errorf("Restart() error = %v, want NotFound", err)
				}
				return
			}

			cluster, err := client.Resource(ClusterGVR).Namespace(namespace).Get(context.Background(), eid, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("failed to get cluster: %v", err)
			}
			restartedAt := cluster.GetAnnotations()[RestartAnnotation]
			if _, err := time.Parse(time.RFC3339, restartedAt); err != nil {
				t.Errorf("annotation %s = %q, want an RFC3339 time", RestartAnnotation, restartedAt)
			}
			if cluster.GetLabels()["app.kubernetes.io/name"] != "sfs-dbaas" {
				t.Errorf("labels changed by the patch: %v", cluster.GetLabels())
			}
		})
	}
}
