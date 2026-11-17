package cluster

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
)

var applicationGVK = schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"}

func testApplication(name, namespace, clusterName, phase string) *unstructured.Unstructured {
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(applicationGVK)
	app.SetName(name)
	app.SetNamespace(namespace)
	app.SetLabels(map[string]string{version.ClusterLabel: clusterName})
	app.Object["operation"] = map[string]interface{}{"sync": map[string]interface{}{}}
	app.Object["status"] = map[string]interface{}{
		"health":         map[string]interface{}{"status": "Progressing"},
		"operationState": map[string]interface{}{"phase": phase},
	}
	return app
}

// TestAbortInProgressSyncs checks that a Running operation is moved to Terminating and its
// .operation is cleared. The Application CRD has no status subresource, so the phase must be
// written on the main resource: the fake client, like the API server, rejects Status() writes
// for types not registered with a status subresource.
func TestAbortInProgressSyncs(t *testing.T) {
	const namespace = "superphenix-system"
	cluster := &operatorv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c1"}}

	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(applicationGVK, meta.RESTScopeNamespace)

	c := fake.NewClientBuilder().
		WithRESTMapper(mapper).
		WithObjects(
			testApplication("c1", namespace, "", operationPhaseRunning),          // root Application
			testApplication("c1-cilium", namespace, "c1", operationPhaseRunning), // stuck sub-application
			testApplication("c1-coredns", namespace, "c1", "Succeeded"),          // finished sub-application
			testApplication("c2-cilium", namespace, "c2", operationPhaseRunning), // other cluster
		).
		Build()

	r := &Reconciler{Client: c, OperatorNamespace: namespace}
	if err := r.abortInProgressSyncs(context.Background(), cluster); err != nil {
		t.Fatalf("abortInProgressSyncs() error = %v", err)
	}

	tests := []struct {
		name          string
		wantPhase     string
		wantOperation bool
	}{
		{name: "c1", wantPhase: operationPhaseTerminating, wantOperation: false},
		{name: "c1-cilium", wantPhase: operationPhaseTerminating, wantOperation: false},
		{name: "c1-coredns", wantPhase: "Succeeded", wantOperation: true},
		{name: "c2-cilium", wantPhase: operationPhaseRunning, wantOperation: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &unstructured.Unstructured{}
			app.SetGroupVersionKind(applicationGVK)
			if err := c.Get(context.Background(), types.NamespacedName{Name: tt.name, Namespace: namespace}, app); err != nil {
				t.Fatal(err)
			}
			phase, _, _ := unstructured.NestedString(app.Object, "status", "operationState", "phase")
			if phase != tt.wantPhase {
				t.Errorf("phase = %q, want %q", phase, tt.wantPhase)
			}
			if _, hasOperation := app.Object["operation"]; hasOperation != tt.wantOperation {
				t.Errorf("operation present = %v, want %v", hasOperation, tt.wantOperation)
			}
			health, _, _ := unstructured.NestedString(app.Object, "status", "health", "status")
			if health != "Progressing" {
				t.Errorf("health = %q, want other status fields preserved", health)
			}
		})
	}
}
