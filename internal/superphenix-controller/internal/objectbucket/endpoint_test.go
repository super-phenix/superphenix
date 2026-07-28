package objectbucket

import (
	"context"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestComposeEndpoint(t *testing.T) {
	tests := []struct {
		name   string
		host   string
		port   int64
		useTls bool
		want   string
	}{
		{"http default port dropped", "s3.example.com", 80, false, "http://s3.example.com"},
		{"http custom port kept", "s3.example.com", 8080, false, "http://s3.example.com:8080"},
		{"https default port dropped", "s3.example.com", 443, true, "https://s3.example.com"},
		{"https on port 80 keeps port", "s3.example.com", 80, true, "https://s3.example.com:80"},
		{"https custom port kept", "s3.example.com", 8443, true, "https://s3.example.com:8443"},
		{"http on port 443 keeps port", "s3.example.com", 443, false, "http://s3.example.com:443"},
		{"zero port dropped", "s3.example.com", 0, false, "http://s3.example.com"},
		{"empty host is empty", "", 80, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := composeEndpoint(tt.host, tt.port, tt.useTls); got != tt.want {
				t.Errorf("composeEndpoint(%q, %d, %v) = %q, want %q", tt.host, tt.port, tt.useTls, got, tt.want)
			}
		})
	}
}

func TestObScheme(t *testing.T) {
	const storeNs = "rook-ceph"

	tests := []struct {
		name      string
		storeName string // additionalState.objectStoreName ("" = no store ref)
		store     *unstructured.Unstructured
		noWatcher bool
		port      int64
		wantTls   bool
		wantPort  int64
	}{
		{
			name:      "advertiseEndpoint useTls true keeps OB port",
			storeName: "store",
			store:     newCephObjectStore(storeNs, "store", map[string]interface{}{"dnsName": "s3.example.com", "port": int64(80), "useTls": true}, 0),
			port:      80,
			wantTls:   true,
			wantPort:  80,
		},
		{
			name:      "advertiseEndpoint useTls false wins over port 443",
			storeName: "store",
			store:     newCephObjectStore(storeNs, "store", map[string]interface{}{"dnsName": "s3.example.com", "port": int64(443), "useTls": false}, 0),
			port:      443,
			wantTls:   false,
			wantPort:  443,
		},
		{
			name:      "securePort preferred over OB http port",
			storeName: "store",
			store:     newCephObjectStore(storeNs, "store", nil, 443),
			port:      80,
			wantTls:   true,
			wantPort:  443,
		},
		{
			name:      "custom securePort preferred over OB http port",
			storeName: "store",
			store:     newCephObjectStore(storeNs, "store", nil, 8443),
			port:      80,
			wantTls:   true,
			wantPort:  8443,
		},
		{
			name:      "securePort matching OB port",
			storeName: "store",
			store:     newCephObjectStore(storeNs, "store", nil, 443),
			port:      443,
			wantTls:   true,
			wantPort:  443,
		},
		{
			name:      "no securePort stays http",
			storeName: "store",
			store:     newCephObjectStore(storeNs, "store", nil, 0),
			port:      443,
			wantTls:   false,
			wantPort:  443,
		},
		{
			name:      "old namespace-prefixed store name resolves",
			storeName: storeNs + ".store",
			store:     newCephObjectStore(storeNs, "store", nil, 443),
			port:      80,
			wantTls:   true,
			wantPort:  443,
		},
		{
			name:      "store missing falls back to 443 heuristic (https)",
			storeName: "store",
			port:      443,
			wantTls:   true,
			wantPort:  443,
		},
		{
			name:      "store missing falls back to 443 heuristic (http)",
			storeName: "store",
			port:      80,
			wantTls:   false,
			wantPort:  80,
		},
		{
			name:      "store watcher absent falls back to 443 heuristic",
			storeName: "store",
			noWatcher: true,
			port:      443,
			wantTls:   true,
			wantPort:  443,
		},
		{
			name:     "no store ref falls back to 443 heuristic",
			port:     443,
			wantTls:  true,
			wantPort: 443,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ob := newOB("ob-test", "s3.example.com", tt.port, "", "")
			if tt.storeName != "" {
				withStoreRef(ob, storeNs, tt.storeName)
			}

			if tt.noWatcher {
				clearWatcher(t, informers.CephObjectStore)
			} else {
				stores := []*unstructured.Unstructured{}
				if tt.store != nil {
					stores = append(stores, tt.store)
				}
				setFakeWatcher(t, informers.CephObjectStore, stores...)
			}

			useTls, port := obScheme(context.Background(), ob, tt.port)
			if useTls != tt.wantTls || port != tt.wantPort {
				t.Errorf("obScheme() = (%v, %d), want (%v, %d)", useTls, port, tt.wantTls, tt.wantPort)
			}
		})
	}
}

func TestResolveOBEndpoint(t *testing.T) {
	const obName = "ob-test"
	const storeNs = "rook-ceph"

	tests := []struct {
		name           string
		seedOB         bool
		link           bool
		store          *unstructured.Unstructured
		wantEndpoint   string
		wantRegion     string
		wantBucketName string
	}{
		{
			name:           "OB present, no store: port heuristic",
			seedOB:         true,
			link:           true,
			wantEndpoint:   "http://s3.example.com",
			wantRegion:     "us-east-1",
			wantBucketName: "bucket-abc",
		},
		{
			name:           "OB present, store declares TLS on port 80",
			seedOB:         true,
			link:           true,
			store:          newCephObjectStore(storeNs, "store", map[string]interface{}{"dnsName": "s3.example.com", "port": int64(80), "useTls": true}, 0),
			wantEndpoint:   "https://s3.example.com:80",
			wantRegion:     "us-east-1",
			wantBucketName: "bucket-abc",
		},
		{
			name:           "OB on http port, store securePort preferred",
			seedOB:         true,
			link:           true,
			store:          newCephObjectStore(storeNs, "store", nil, 443),
			wantEndpoint:   "https://s3.example.com",
			wantRegion:     "us-east-1",
			wantBucketName: "bucket-abc",
		},
		{
			name: "objectBucketName empty",
			link: false,
		},
		{
			name: "OB missing",
			link: true, // references obName but no OB object exists
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := testNamespace(t)
			obc := newOBC(t, namespace, "rook-ceph-bucket", nil, nil)
			if tt.link {
				withObjectBucketName(obc, obName)
			}

			objs := []runtime.Object{obc}
			if tt.seedOB {
				ob := newOB(obName, "s3.example.com", 80, "us-east-1", "bucket-abc")
				if tt.store != nil {
					withStoreRef(ob, storeNs, "store")
				}
				objs = append(objs, ob)
			}
			setFakeDynamicClient(t, objs...)

			stores := []*unstructured.Unstructured{}
			if tt.store != nil {
				stores = append(stores, tt.store)
			}
			setFakeWatcher(t, informers.CephObjectStore, stores...)

			got := resolveOBEndpoint(context.Background(), obc)
			if got.Endpoint != tt.wantEndpoint {
				t.Errorf("Endpoint = %q, want %q", got.Endpoint, tt.wantEndpoint)
			}
			if got.Region != tt.wantRegion {
				t.Errorf("Region = %q, want %q", got.Region, tt.wantRegion)
			}
			if got.BucketName != tt.wantBucketName {
				t.Errorf("BucketName = %q, want %q", got.BucketName, tt.wantBucketName)
			}
		})
	}
}
