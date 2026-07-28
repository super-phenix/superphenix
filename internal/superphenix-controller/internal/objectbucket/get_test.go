package objectbucket

import (
	"context"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGetBucket(t *testing.T) {
	const obName = "ob-single"

	tests := []struct {
		name             string
		seedOB           bool
		externalEndpoint string
		wantEndpoint     string
	}{
		{"endpoint from OB", true, "", "http://s3.example.com"},
		{"fallback to external when no OB", false, "https://s3.ext", "https://s3.ext"},
		{"empty when no OB and no external", false, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setS3Config(t, map[string]string{"standard": "rook-ceph-bucket"}, "", 0, tt.externalEndpoint)

			namespace := testNamespace(t)
			obc := withObjectBucketName(newOBC(t, namespace, "rook-ceph-bucket", nil, nil), obName)

			objs := []runtime.Object{obc}
			if tt.seedOB {
				objs = append(objs, newOB(obName, "s3.example.com", 80, "", ""))
			}
			setFakeDynamicClient(t, objs...)

			got, err := GetBucket(context.Background(), namespace, testEffectiveId(t))
			if err != nil {
				t.Fatalf("GetBucket() error = %v", err)
			}
			if got.Bucket.Endpoint != tt.wantEndpoint {
				t.Errorf("Endpoint = %q, want %q", got.Bucket.Endpoint, tt.wantEndpoint)
			}
		})
	}
}

// listOBC builds a minimal namespaced OBC for list assertions.
func listOBC(namespace, bucketName, obName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "objectbucket.io/v1alpha1",
			"kind":       "ObjectBucketClaim",
			"metadata": map[string]interface{}{
				"name":      NamePrefix + bucketName,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"bucketName":       bucketName,
				"storageClassName": "rook-ceph-bucket",
				"objectBucketName": obName,
			},
		},
	}
}

func TestGetOBEndpoint(t *testing.T) {
	const storeNs = "rook-ceph"

	tests := []struct {
		name      string
		obName    string
		noWatcher bool
		obs       []*unstructured.Unstructured
		stores    []*unstructured.Unstructured
		want      string
	}{
		{
			name:   "OB without store ref uses port heuristic",
			obName: "ob-plain",
			obs:    []*unstructured.Unstructured{newOB("ob-plain", "s3.example.com", 80, "", "")},
			want:   "http://s3.example.com",
		},
		{
			name:   "OB with TLS store ref",
			obName: "ob-tls",
			obs: []*unstructured.Unstructured{
				withStoreRef(newOB("ob-tls", "s3.example.com", 80, "", ""), storeNs, "store"),
			},
			stores: []*unstructured.Unstructured{
				newCephObjectStore(storeNs, "store", map[string]interface{}{"dnsName": "s3.example.com", "port": int64(80), "useTls": true}, 0),
			},
			want: "https://s3.example.com:80",
		},
		{
			name: "empty obName",
			obs:  []*unstructured.Unstructured{newOB("ob-plain", "s3.example.com", 80, "", "")},
			want: "",
		},
		{
			name:      "OB watcher absent",
			obName:    "ob-plain",
			noWatcher: true,
			want:      "",
		},
		{
			name:   "OB not in cache",
			obName: "ob-missing",
			obs:    []*unstructured.Unstructured{newOB("ob-plain", "s3.example.com", 80, "", "")},
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.noWatcher {
				clearWatcher(t, informers.ObjectBucket)
			} else {
				setFakeWatcher(t, informers.ObjectBucket, tt.obs...)
			}
			setFakeWatcher(t, informers.CephObjectStore, tt.stores...)

			if got := getOBEndpoint(context.Background(), tt.obName); got != tt.want {
				t.Errorf("getOBEndpoint(%q) = %q, want %q", tt.obName, got, tt.want)
			}
		})
	}
}

func TestListBuckets(t *testing.T) {
	namespace := testNamespace(t)

	obcA := listOBC(namespace, "bucket-aaa", "ob-aaa")
	obcB := listOBC(namespace, "bucket-bbb", "ob-bbb") // no OB -> fallback
	obcC := listOBC(namespace, "bucket-ccc", "ob-ccc")
	obcD := listOBC(namespace, "bucket-ddd", "ob-ddd")
	obA := newOB("ob-aaa", "s3.a.example", 80, "", "")

	// C and D share one TLS store
	store := newCephObjectStore("rook-ceph", "store", map[string]interface{}{"dnsName": "s3.cd.example", "port": int64(80), "useTls": true}, 0)
	obC := withStoreRef(newOB("ob-ccc", "s3.c.example", 80, "", ""), "rook-ceph", "store")
	obD := withStoreRef(newOB("ob-ddd", "s3.d.example", 8443, "", ""), "rook-ceph", "store")

	tests := []struct {
		name         string
		noOBCWatcher bool
		noOBWatcher  bool
		obcs         []*unstructured.Unstructured
		obs          []*unstructured.Unstructured
		stores       []*unstructured.Unstructured
		wantErr      bool
		want         map[string]string // bucket name -> endpoint
	}{
		{
			name:   "endpoints resolved from cached OBs and stores",
			obcs:   []*unstructured.Unstructured{obcA, obcB, obcC, obcD},
			obs:    []*unstructured.Unstructured{obA, obC, obD},
			stores: []*unstructured.Unstructured{store},
			want: map[string]string{
				"bucket-aaa": "http://s3.a.example",     // no store -> port heuristic
				"bucket-bbb": "https://fallback.ext",    // no OB -> config fallback
				"bucket-ccc": "https://s3.c.example:80", // store useTls on port 80
				"bucket-ddd": "https://s3.d.example:8443",
			},
		},
		{
			name: "empty cache lists nothing",
			want: map[string]string{},
		},
		{
			name: "other namespace excluded",
			obcs: []*unstructured.Unstructured{listOBC("other-namespace", "bucket-other", "ob-other")},
			want: map[string]string{},
		},
		{
			name:        "no OB watcher falls back to configured endpoint",
			noOBWatcher: true,
			obcs:        []*unstructured.Unstructured{obcA},
			want:        map[string]string{"bucket-aaa": "https://fallback.ext"},
		},
		{
			name:         "no OBC watcher errors",
			noOBCWatcher: true,
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setS3Config(t, map[string]string{"standard": "rook-ceph-bucket"}, "", 0, "https://fallback.ext")

			if tt.noOBCWatcher {
				clearWatcher(t, informers.ObjectBucketClaim)
			} else {
				setFakeWatcher(t, informers.ObjectBucketClaim, tt.obcs...)
			}
			if tt.noOBWatcher {
				clearWatcher(t, informers.ObjectBucket)
			} else {
				setFakeWatcher(t, informers.ObjectBucket, tt.obs...)
			}
			setFakeWatcher(t, informers.CephObjectStore, tt.stores...)

			buckets, err := ListBuckets(context.Background(), namespace)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ListBuckets() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ListBuckets() error = %v", err)
			}
			if len(buckets) != len(tt.want) {
				t.Fatalf("got %d buckets, want %d", len(buckets), len(tt.want))
			}

			endpoints := map[string]string{}
			for _, b := range buckets {
				endpoints[b.Bucket.Name] = b.Bucket.Endpoint
			}
			for name, wantEndpoint := range tt.want {
				if endpoints[name] != wantEndpoint {
					t.Errorf("%s endpoint = %q, want %q", name, endpoints[name], wantEndpoint)
				}
			}
		})
	}
}
