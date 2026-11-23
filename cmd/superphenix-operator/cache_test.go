package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
)

func byObjectFor[T any](t *testing.T, opts cache.Options) (cache.ByObject, bool) {
	t.Helper()
	for obj, cfg := range opts.ByObject {
		if _, ok := obj.(T); ok {
			return cfg, true
		}
	}
	return cache.ByObject{}, false
}

func TestCacheOptions(t *testing.T) {
	const namespace = "superphenix-system"
	opts, err := cacheOptions(namespace, true)
	if err != nil {
		t.Fatal(err)
	}
	if opts.DefaultTransform == nil {
		t.Error("managed fields are not stripped from cached objects")
	}

	secrets, _ := byObjectFor[*corev1.Secret](t, opts)
	if secrets.Namespaces != nil {
		t.Errorf("secret cache namespaces = %v, want all namespaces", secrets.Namespaces)
	}
	if secrets.Label == nil || secrets.Label.Matches(labels.Set{"owner": "helm"}) || !secrets.Label.Matches(labels.Set{}) {
		t.Errorf("secret cache label = %v, want only Helm release Secrets excluded", secrets.Label)
	}

	configMaps, _ := byObjectFor[*corev1.ConfigMap](t, opts)
	if _, ok := configMaps.Namespaces[namespace]; !ok || len(configMaps.Namespaces) != 1 {
		t.Errorf("configmap cache namespaces = %v, want only %s", configMaps.Namespaces, namespace)
	}

	apps, ok := byObjectFor[*metav1.PartialObjectMetadata](t, opts)
	if !ok {
		t.Fatal("no Application cache settings")
	}
	if apps.Namespaces != nil {
		t.Errorf("application cache namespaces = %v, want all namespaces", apps.Namespaces)
	}
	if apps.Label == nil || !apps.Label.Matches(labels.Set{version.ManagedLabel: "true"}) || apps.Label.Matches(labels.Set{}) {
		t.Errorf("application cache label = %v, want managed Applications only", apps.Label)
	}
}

func TestCacheOptionsWithoutApplicationsOrNamespace(t *testing.T) {
	opts, err := cacheOptions("", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := byObjectFor[*metav1.PartialObjectMetadata](t, opts); ok {
		t.Error("Application cache settings registered before the CRDs exist")
	}
	if configMaps, _ := byObjectFor[*corev1.ConfigMap](t, opts); configMaps.Namespaces != nil {
		t.Errorf("configmap cache namespaces = %v, want all namespaces", configMaps.Namespaces)
	}
}

func TestManagerClientOptions(t *testing.T) {
	opts := managerClientOptions()
	if opts.Cache == nil {
		t.Fatal("client cache options are nil")
	}
	if opts.Cache.Unstructured {
		t.Error("unstructured reads are served from the metadata-only, label-filtered cache")
	}
	secretUncached := false
	for _, obj := range opts.Cache.DisableFor {
		if _, ok := obj.(*corev1.Secret); ok {
			secretUncached = true
		}
	}
	if !secretUncached {
		t.Error("secret reads are served from the metadata-only cache")
	}
}
