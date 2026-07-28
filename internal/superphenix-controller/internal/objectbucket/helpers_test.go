package objectbucket

import (
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	"testing"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
)

const (
	testOrgId     = "11111111-1111-1111-1111-111111111111"
	testProjectId = "22222222-2222-2222-2222-222222222222"
	testLocalId   = "33333333-3333-3333-3333-333333333333"
)

func testMetadata(t *testing.T) spxId.Metadata {
	t.Helper()
	m := spxId.Metadata{}
	if err := m.GenerateMetadata(testProjectId, testOrgId, testLocalId); err != nil {
		t.Fatalf("failed to generate metadata: %v", err)
	}
	return m
}

func testNamespace(t *testing.T) string {
	t.Helper()
	m := testMetadata(t)
	return m.GetProjectID()
}

func testEffectiveId(t *testing.T) string {
	t.Helper()
	m := testMetadata(t)
	return m.GetResourceEffectiveID()
}

func setS3Config(t *testing.T, mapping map[string]string, maxBucketSize string, maxBucketObjects uint64, externalEndpoint string) {
	t.Helper()
	old := config.Global.S3
	config.Global.S3.StorageClassMapping = mapping
	config.Global.S3.MaxBucketSize = maxBucketSize
	config.Global.S3.MaxBucketObjects = maxBucketObjects
	config.Global.S3.ExternalEndpoint = externalEndpoint
	t.Cleanup(func() { config.Global.S3 = old })
}

func setFakeDynamicClient(t *testing.T, objects ...runtime.Object) {
	t.Helper()
	old := config.DynamicClientSet
	config.DynamicClientSet = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			ObjectBucketClaimGVR: "ObjectBucketClaimList",
			ObjectBucketGVR:      "ObjectBucketList",
			CephObjectStoreGVR:   "CephObjectStoreList",
		},
		objects...,
	)
	t.Cleanup(func() { config.DynamicClientSet = old })
}

// setFakeWatcher seeds the informer registry with a prefilled indexer, as the
// dynamic informer would build it, and restores the previous entry on cleanup.
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

// newOBC builds an OBC as CreateBucket would, with optional label overrides.
func newOBC(t *testing.T, namespace, storageClassName string, additionalConfig map[string]interface{}, labelOverrides map[string]string) *unstructured.Unstructured {
	t.Helper()
	m := testMetadata(t)
	eid := m.GetResourceEffectiveID()

	spec := map[string]interface{}{
		"bucketName":       eid,
		"storageClassName": storageClassName,
	}
	if len(additionalConfig) > 0 {
		spec["additionalConfig"] = additionalConfig
	}

	obc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "objectbucket.io/v1alpha1",
			"kind":       "ObjectBucketClaim",
			"metadata": map[string]interface{}{
				"name":      NamePrefix + eid,
				"namespace": namespace,
			},
			"spec": spec,
		},
	}

	labels := m.GetLabels()
	for k, v := range labelOverrides {
		labels[k] = v
	}
	obc.SetLabels(labels)

	return obc
}

// newOB builds a cluster-scoped ObjectBucket carrying an S3 endpoint.
func newOB(name, host string, port int64, region, bucketName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "objectbucket.io/v1alpha1",
			"kind":       "ObjectBucket",
			"metadata": map[string]interface{}{
				"name": name,
			},
			"spec": map[string]interface{}{
				"endpoint": map[string]interface{}{
					"bucketHost": host,
					"bucketPort": port,
					"region":     region,
					"bucketName": bucketName,
				},
			},
		},
	}
}

// withObjectBucketName sets spec.objectBucketName so the OBC binds to an OB.
func withObjectBucketName(obc *unstructured.Unstructured, obName string) *unstructured.Unstructured {
	_ = unstructured.SetNestedField(obc.Object, obName, "spec", "objectBucketName")
	return obc
}

// withStoreRef sets the OB's additionalState reference to its CephObjectStore.
func withStoreRef(ob *unstructured.Unstructured, storeNamespace, storeName string) *unstructured.Unstructured {
	_ = unstructured.SetNestedField(ob.Object, storeName, "spec", "additionalState", "objectStoreName")
	_ = unstructured.SetNestedField(ob.Object, storeNamespace, "spec", "additionalState", "objectStoreNamespace")
	return ob
}

// newCephObjectStore builds a namespaced store. advertise (dnsName/port/useTls)
// is optional; securePort fills spec.gateway.securePort.
func newCephObjectStore(namespace, name string, advertise map[string]interface{}, securePort int64) *unstructured.Unstructured {
	spec := map[string]interface{}{
		"gateway": map[string]interface{}{
			"port":       int64(80),
			"securePort": securePort,
		},
	}
	if advertise != nil {
		spec["hosting"] = map[string]interface{}{
			"advertiseEndpoint": advertise,
		}
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "ceph.rook.io/v1",
			"kind":       "CephObjectStore",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": spec,
		},
	}
}

func uintPtr(v uint64) *uint64 {
	return &v
}
