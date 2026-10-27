package dbaas

import (
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
)

const (
	testOrgId        = "11111111-1111-1111-1111-111111111111"
	testProjectId    = "22222222-2222-2222-2222-222222222222"
	testOtherProject = "44444444-4444-4444-4444-444444444444"
	testLocalId      = "33333333-3333-3333-3333-333333333333"
	testOtherLocalId = "55555555-5555-5555-5555-555555555555"
	testVIP          = "198.18.0.10"
)

func testMetadata(t *testing.T, projectId string) spxId.Metadata {
	t.Helper()
	m := spxId.Metadata{}
	if err := m.GenerateMetadata(projectId, testOrgId, testLocalId); err != nil {
		t.Fatalf("failed to generate metadata: %v", err)
	}
	return m
}

func testNamespace(t *testing.T) string {
	t.Helper()
	m := testMetadata(t, testProjectId)
	return m.GetProjectID()
}

func testEffectiveId(t *testing.T) string {
	t.Helper()
	m := testMetadata(t, testProjectId)
	return m.GetResourceEffectiveID()
}

func setFakeDynamicClient(t *testing.T, objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	t.Helper()
	old := config.DynamicClientSet
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{ClusterGVR: "ClusterList"},
		objects...,
	)
	config.DynamicClientSet = client
	t.Cleanup(func() { config.DynamicClientSet = old })
	return client
}

// setFakeWatcher seeds the informer registry with a prefilled indexer and restores the previous entry on cleanup.
func setFakeWatcher(t *testing.T, resource string, objects ...*unstructured.Unstructured) {
	t.Helper()
	idx := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, obj := range objects {
		if err := idx.Add(obj); err != nil {
			t.Fatalf("failed to seed fake watcher: %v", err)
		}
	}
	old, had := informers.WatcherSet[resource]
	informers.WatcherSet[resource] = informers.Watcher{Indexer: idx}
	t.Cleanup(func() {
		if had {
			informers.WatcherSet[resource] = old
		} else {
			delete(informers.WatcherSet, resource)
		}
	})
}

// clearWatcher removes a watcher to simulate an uninitialized informer.
func clearWatcher(t *testing.T, resource string) {
	t.Helper()
	old, had := informers.WatcherSet[resource]
	if !had {
		return
	}
	delete(informers.WatcherSet, resource)
	t.Cleanup(func() { informers.WatcherSet[resource] = old })
}

// newCluster builds a Cluster as the sfs-dbaas chart renders it, labelled for projectId,
// in the namespace of the test project.
func newCluster(t *testing.T, projectId string) *unstructured.Unstructured {
	t.Helper()
	m := testMetadata(t, projectId)

	cluster := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "postgresql.cnpg.io/v1",
			"kind":       "Cluster",
			"metadata": map[string]any{
				"name":      testEffectiveId(t),
				"namespace": testNamespace(t),
			},
			"spec": map[string]any{
				"instances": int64(2),
				"imageName": "ghcr.io/cloudnative-pg/postgresql:17",
				"storage": map[string]any{
					"size":         "10Gi",
					"storageClass": "ceph-block",
				},
				"resources": map[string]any{
					"requests": map[string]any{"cpu": "1", "memory": "1Gi"},
					"limits":   map[string]any{"cpu": "1", "memory": "1Gi"},
				},
				"postgresql": map[string]any{
					"parameters": map[string]any{"max_connections": "100"},
				},
			},
			"status": map[string]any{
				"phase":          "Cluster in healthy state",
				"instances":      int64(2),
				"readyInstances": int64(2),
				"currentPrimary": testEffectiveId(t) + "-1",
				"conditions": []any{
					map[string]any{
						"type":               "Ready",
						"status":             "True",
						"reason":             "ClusterIsReady",
						"message":            "Cluster is Ready",
						"lastTransitionTime": "2026-10-16T10:00:00Z",
					},
				},
			},
		},
	}

	labels := m.GetLabels()
	labels[spxId.SpxLabelResourceName] = "orders"
	labels["app.kubernetes.io/name"] = "sfs-dbaas"
	cluster.SetLabels(labels)

	return cluster
}

// newLBRule builds the cluster-scoped SwitchLBRule the chart renders for a database.
func newLBRule(namespace, localId string, ports ...int64) *unstructured.Unstructured {
	specPorts := make([]any, 0, len(ports))
	for _, port := range ports {
		specPorts = append(specPorts, map[string]any{"port": port, "targetPort": port, "protocol": "TCP"})
	}

	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "kubeovn.io/v1",
			"kind":       "SwitchLBRule",
			"metadata": map[string]any{
				"name": "lb",
				"labels": map[string]any{
					spxId.SpxLabelProjectID:       namespace,
					spxId.SpxLabelResourceLocalID: LBLocalIDPrefix + localId,
				},
			},
			"spec": map[string]any{
				"vip":   testVIP,
				"ports": specPorts,
			},
		},
	}
}
