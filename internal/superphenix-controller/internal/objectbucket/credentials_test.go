package objectbucket

import (
	"context"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
)

func TestGetCredentials(t *testing.T) {
	const obName = "ob-creds"

	tests := []struct {
		name             string
		externalEndpoint string
		seedOB           bool
		seedStore        bool
		noOBC            bool
		noSecret         bool
		wantErr          bool
		wantNotFound     bool
		wantEndpoint     string
		wantRegion       string
	}{
		{
			name:         "endpoint and region from OB",
			seedOB:       true,
			wantEndpoint: "http://rook-ceph-rgw.svc:8080",
			wantRegion:   "us-east-1",
		},
		{
			name:         "store declares TLS",
			seedOB:       true,
			seedStore:    true,
			wantEndpoint: "https://rook-ceph-rgw.svc:8080",
			wantRegion:   "us-east-1",
		},
		{
			name:             "external endpoint fallback when no OB",
			externalEndpoint: "https://s3.az1.superphenix.net",
			seedOB:           false,
			wantEndpoint:     "https://s3.az1.superphenix.net",
		},
		{
			name:         "OBC missing",
			noOBC:        true,
			wantErr:      true,
			wantNotFound: true,
		},
		{
			name:     "Secret missing",
			seedOB:   true,
			noSecret: true,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setS3Config(t, map[string]string{"standard": "rook-ceph-bucket"}, "", 0, tt.externalEndpoint)

			namespace := testNamespace(t)
			eid := testEffectiveId(t)
			name := NamePrefix + eid

			var objs []runtime.Object
			if !tt.noOBC {
				objs = append(objs, withObjectBucketName(newOBC(t, namespace, "rook-ceph-bucket", nil, nil), obName))
			}
			stores := []*unstructured.Unstructured{}
			if tt.seedOB {
				ob := newOB(obName, "rook-ceph-rgw.svc", 8080, "us-east-1", eid)
				if tt.seedStore {
					withStoreRef(ob, "rook-ceph", "store")
					stores = append(stores, newCephObjectStore("rook-ceph", "store", map[string]interface{}{"dnsName": "rook-ceph-rgw.svc", "port": int64(8080), "useTls": true}, 0))
				}
				objs = append(objs, ob)
			}
			setFakeDynamicClient(t, objs...)
			setFakeWatcher(t, informers.CephObjectStore, stores...)

			fakeClient := fake.NewClientset()
			if !tt.noSecret {
				_, _ = fakeClient.CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
					Data: map[string][]byte{
						"AWS_ACCESS_KEY_ID":     []byte("ACCESSKEY"),
						"AWS_SECRET_ACCESS_KEY": []byte("SECRETKEY"),
					},
				}, metav1.CreateOptions{})
			}
			oldClient := config.K8sClient
			config.K8sClient = fakeClient
			t.Cleanup(func() { config.K8sClient = oldClient })

			credentials, err := GetCredentials(context.Background(), namespace, eid)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.wantNotFound && !apierrors.IsNotFound(err) {
					t.Errorf("GetCredentials() error = %v, want NotFound", err)
				}
				return
			}

			if credentials.Endpoint != tt.wantEndpoint {
				t.Errorf("Endpoint = %q, want %q", credentials.Endpoint, tt.wantEndpoint)
			}
			if credentials.BucketName != eid {
				t.Errorf("BucketName = %q, want %q", credentials.BucketName, eid)
			}
			if credentials.Region != tt.wantRegion {
				t.Errorf("Region = %q, want %q", credentials.Region, tt.wantRegion)
			}
			if credentials.AccessKeyID != "ACCESSKEY" {
				t.Errorf("AccessKeyID = %q, want %q", credentials.AccessKeyID, "ACCESSKEY")
			}
			if credentials.SecretAccessKey != "SECRETKEY" {
				t.Errorf("SecretAccessKey = %q, want %q", credentials.SecretAccessKey, "SECRETKEY")
			}
		})
	}
}
