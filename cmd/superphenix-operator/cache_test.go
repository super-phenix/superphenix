package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

func byObjectFor[T any](t *testing.T, opts cache.Options) cache.ByObject {
	t.Helper()
	for obj, cfg := range opts.ByObject {
		if _, ok := obj.(T); ok {
			return cfg
		}
	}
	var zero T
	t.Fatalf("no cache settings for %T", zero)
	return cache.ByObject{}
}

func TestCoreCacheOptions(t *testing.T) {
	const namespace = "superphenix-system"
	opts, err := coreCacheOptions(namespace)
	if err != nil {
		t.Fatal(err)
	}
	if opts.DefaultTransform == nil {
		t.Error("managed fields are not stripped from cached objects")
	}

	secrets := byObjectFor[*corev1.Secret](t, opts)
	if secrets.Namespaces != nil {
		t.Errorf("secret cache namespaces = %v, want all namespaces", secrets.Namespaces)
	}
	if secrets.Label.Matches(labels.Set{"owner": "helm"}) {
		t.Error("helm release secrets are cached")
	}
	if !secrets.Label.Matches(labels.Set{}) {
		t.Error("unlabeled secrets are not cached")
	}
	if secrets.Transform == nil {
		t.Fatal("secret transform is nil")
	}

	configMaps := byObjectFor[*corev1.ConfigMap](t, opts)
	if _, ok := configMaps.Namespaces[namespace]; !ok || len(configMaps.Namespaces) != 1 {
		t.Errorf("configmap cache namespaces = %v, want only %s", configMaps.Namespaces, namespace)
	}
}

func TestCoreCacheOptionsWithoutNamespace(t *testing.T) {
	opts, err := coreCacheOptions("")
	if err != nil {
		t.Fatal(err)
	}

	secrets := byObjectFor[*corev1.Secret](t, opts)
	if secrets.Namespaces != nil {
		t.Errorf("secret cache namespaces = %v, want all namespaces", secrets.Namespaces)
	}
	if secrets.Label.Matches(labels.Set{"owner": "helm"}) {
		t.Error("helm release secrets are cached")
	}
	if configMaps := byObjectFor[*corev1.ConfigMap](t, opts); configMaps.Namespaces != nil {
		t.Errorf("configmap cache namespaces = %v, want all namespaces", configMaps.Namespaces)
	}
}

func TestTransformStripSecretData(t *testing.T) {
	secret := &corev1.Secret{
		Data:       map[string][]byte{"token": []byte("super-secret")},
		StringData: map[string]string{"password": "hunter2"},
	}
	secret.SetManagedFields(nil)

	out, err := transformStripSecretData(secret)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := out.(*corev1.Secret)
	if !ok {
		t.Fatalf("got %T, want *corev1.Secret", out)
	}
	if got.Data != nil || got.StringData != nil {
		t.Errorf("secret payloads still present: data=%v stringData=%v", got.Data, got.StringData)
	}
}

func TestArgoApplicationCacheOptions(t *testing.T) {
	sel, err := labels.Parse("operator.superphenix.net/managed=true")
	if err != nil {
		t.Fatal(err)
	}

	opts := argoApplicationCacheOptions(sel)
	if !opts.Label.Matches(labels.Set{"operator.superphenix.net/managed": "true"}) {
		t.Error("managed applications are not selected")
	}
	if opts.Label.Matches(labels.Set{}) {
		t.Error("unmanaged applications are selected")
	}
	if opts.Namespaces != nil {
		t.Errorf("application cache namespaces = %v, want all namespaces (project GitOps apps)", opts.Namespaces)
	}
	if opts.Transform == nil {
		t.Fatal("application transform is nil")
	}
}

func TestTransformStripArgoApplicationStatus(t *testing.T) {
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Application",
	})
	_ = unstructured.SetNestedMap(app.Object, map[string]interface{}{
		"health": map[string]interface{}{
			"status":  "Healthy",
			"message": "all good",
		},
		"sync": map[string]interface{}{
			"status": "Synced",
			"comparedTo": map[string]interface{}{
				"source": map[string]interface{}{"repoURL": "https://example.com"},
			},
		},
		"reconciledAt": "2026-01-02T03:04:05Z",
		"resources": []interface{}{
			map[string]interface{}{"kind": "Deployment", "name": "big"},
			map[string]interface{}{"kind": "Service", "name": "bigger"},
		},
		"history": []interface{}{
			map[string]interface{}{"id": float64(1), "revision": "1.0.0"},
		},
		"operationState": map[string]interface{}{
			"phase":      "Running",
			"startedAt":  "2026-01-02T03:00:00Z",
			"finishedAt": "2026-01-02T03:01:00Z",
			"syncResult": map[string]interface{}{
				"resources": []interface{}{map[string]interface{}{"name": "big"}},
			},
		},
	}, "status")

	out, err := transformStripArgoApplicationStatus(app)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := out.(*unstructured.Unstructured)
	if !ok {
		t.Fatalf("got %T, want *unstructured.Unstructured", out)
	}

	status, found, err := unstructured.NestedMap(got.Object, "status")
	if err != nil || !found {
		t.Fatalf("status missing after transform: found=%v err=%v", found, err)
	}
	if _, ok := status["resources"]; ok {
		t.Error("status.resources was not stripped")
	}
	if _, ok := status["history"]; ok {
		t.Error("status.history was not stripped")
	}
	health, _, _ := unstructured.NestedString(got.Object, "status", "health", "status")
	if health != "Healthy" {
		t.Errorf("health.status = %q, want Healthy", health)
	}
	sync, _, _ := unstructured.NestedString(got.Object, "status", "sync", "status")
	if sync != "Synced" {
		t.Errorf("sync.status = %q, want Synced", sync)
	}
	if _, found, _ := unstructured.NestedFieldNoCopy(got.Object, "status", "sync", "comparedTo"); found {
		t.Error("sync.comparedTo was not stripped")
	}
	phase, _, _ := unstructured.NestedString(got.Object, "status", "operationState", "phase")
	if phase != "Running" {
		t.Errorf("operationState.phase = %q, want Running", phase)
	}
	if _, found, _ := unstructured.NestedFieldNoCopy(got.Object, "status", "operationState", "syncResult"); found {
		t.Error("operationState.syncResult was not stripped")
	}
}
