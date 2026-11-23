package cluster

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
)

const testNamespace = "superphenix-system"

var applicationGVK = schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"}

// testApplication builds an Application of cluster c1. An empty phase means no operation.
func testApplication(name, phase string, startedAt time.Time) client.Object {
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(applicationGVK)
	app.SetName(name)
	app.SetNamespace(testNamespace)
	if name != "c1" { // the root Application is fetched by name, children by label
		app.SetLabels(map[string]string{version.ClusterLabel: "c1"})
	}
	status := map[string]interface{}{"health": map[string]interface{}{"status": "Progressing"}}
	if phase != "" {
		app.Object["operation"] = map[string]interface{}{"sync": map[string]interface{}{"revision": "old"}}
		status["operationState"] = map[string]interface{}{
			"phase":     phase,
			"startedAt": startedAt.UTC().Format(time.RFC3339),
		}
	}
	app.Object["status"] = status
	return app
}

type appState struct {
	phase     string
	operation map[string]interface{}
}

func runPeriodicSyncOn(t *testing.T, objs ...client.Object) (bool, map[string]appState) {
	t.Helper()
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(applicationGVK, meta.RESTScopeNamespace)
	c := fake.NewClientBuilder().WithRESTMapper(mapper).WithObjects(objs...).Build()

	r := &Reconciler{Client: c, OperatorNamespace: testNamespace, SyncPeriod: 5 * time.Minute, SyncTimeout: 15 * time.Minute}
	triggered := r.runPeriodicSync(context.Background(), &operatorv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c1"}})

	states := map[string]appState{}
	for _, obj := range objs {
		app := &unstructured.Unstructured{}
		app.SetGroupVersionKind(applicationGVK)
		if err := c.Get(context.Background(), types.NamespacedName{Name: obj.GetName(), Namespace: testNamespace}, app); err != nil {
			t.Fatal(err)
		}
		if health, _, _ := unstructured.NestedString(app.Object, "status", "health", "status"); health != "Progressing" {
			t.Errorf("%s: health = %q, want other status fields preserved", obj.GetName(), health)
		}
		phase, _, _ := unstructured.NestedString(app.Object, "status", "operationState", "phase")
		operation, _, _ := unstructured.NestedMap(app.Object, "operation")
		states[obj.GetName()] = appState{phase: phase, operation: operation}
	}
	return triggered, states
}

func TestRunPeriodicSyncTriggersIdleApplications(t *testing.T) {
	triggered, states := runPeriodicSyncOn(t,
		testApplication("c1", "", time.Time{}),
		testApplication("c1-cilium", "Succeeded", time.Now().Add(-time.Hour)),
	)
	if !triggered {
		t.Fatal("sync not triggered")
	}
	for name, s := range states {
		if s.operation == nil || s.operation["sync"] == nil {
			t.Errorf("%s: operation = %v, want a sync operation", name, s.operation)
		}
	}
}

// A stale Running operation is asked to terminate, keeping .operation for ArgoCD to complete it.
// No replacement sync is written in the same pass: ArgoCD would discard it.
func TestRunPeriodicSyncTerminatesStaleOperation(t *testing.T) {
	triggered, states := runPeriodicSyncOn(t,
		testApplication("c1", operationPhaseRunning, time.Now().Add(-time.Hour)),
		testApplication("c1-cilium", operationPhaseRunning, time.Now()),
		testApplication("c1-coredns", "", time.Time{}),
	)
	if triggered {
		t.Error("sync triggered while an operation is terminating")
	}
	if s := states["c1"]; s.phase != operationPhaseTerminating || s.operation["sync"] == nil {
		t.Errorf("stale root: phase = %q, operation = %v, want Terminating with operation kept", s.phase, s.operation)
	}
	if s := states["c1-cilium"]; s.phase != operationPhaseRunning {
		t.Errorf("recent operation: phase = %q, want Running", s.phase)
	}
	if s := states["c1-coredns"]; s.operation != nil {
		t.Errorf("idle child: operation = %v, want none while another operation is in progress", s.operation)
	}
}

func TestRunPeriodicSyncWaitsForTermination(t *testing.T) {
	triggered, states := runPeriodicSyncOn(t,
		testApplication("c1", operationPhaseTerminating, time.Now().Add(-time.Hour)),
		testApplication("c1-coredns", "", time.Time{}),
	)
	if triggered {
		t.Error("sync triggered while an operation is terminating")
	}
	if s := states["c1-coredns"]; s.operation != nil {
		t.Errorf("idle child: operation = %v, want none", s.operation)
	}
}
