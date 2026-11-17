package main

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
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
