package objectbucket

import (
	"context"
	"reflect"
	"testing"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
)

func TestUpdateBucket(t *testing.T) {
	existingConfig := map[string]interface{}{
		"bucketMaxObjects": "50",
		"bucketPolicy":     `{"old":true}`,
	}

	tests := []struct {
		name                 string
		labelOverrides       map[string]string
		noResource           bool
		config               BucketConfig
		wantErr              bool
		wantNotFound         bool
		wantAdditionalConfig map[string]interface{}
	}{
		{
			name: "replaces additionalConfig",
			config: BucketConfig{
				MaxObjects: uintPtr(200),
				MaxSize:    "20Gi",
			},
			wantAdditionalConfig: map[string]interface{}{
				"bucketMaxObjects": "200",
				"bucketMaxSize":    "20Gi",
			},
		},
		{
			name:   "empty config removes additionalConfig",
			config: BucketConfig{},
		},
		{
			name:           "gitops resource rejected",
			labelOverrides: map[string]string{spxId.SpxLabelGitops: "true"},
			wantErr:        true,
		},
		{
			name:           "cross-project rejected",
			labelOverrides: map[string]string{spxId.SpxLabelProjectID: "spx-other-project"},
			wantErr:        true,
			wantNotFound:   true,
		},
		{
			name:         "not found",
			noResource:   true,
			wantErr:      true,
			wantNotFound: true,
		},
		{
			name:    "maxSize over cap rejected",
			config:  BucketConfig{MaxSize: "10Ti"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setS3Config(t, map[string]string{"standard": "rook-ceph-bucket"}, "1Ti", 0, "")

			namespace := testNamespace(t)
			eid := testEffectiveId(t)

			if tt.noResource {
				setFakeDynamicClient(t)
			} else {
				setFakeDynamicClient(t, newOBC(t, namespace, "rook-ceph-bucket", existingConfig, tt.labelOverrides))
			}

			info := UpdateBucketInfo{}
			info.General.Config = tt.config

			err := info.UpdateBucket(context.Background(), namespace, eid)
			if (err != nil) != tt.wantErr {
				t.Fatalf("UpdateBucket() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.wantNotFound && !apierrors.IsNotFound(err) {
					t.Errorf("UpdateBucket() error = %v, want NotFound", err)
				}
				return
			}

			obc, err := config.DynamicClientSet.Resource(ObjectBucketClaimGVR).Namespace(namespace).Get(context.Background(), NamePrefix+eid, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("OBC not found after update: %v", err)
			}

			bucketName, _, _ := unstructured.NestedString(obc.Object, "spec", "bucketName")
			if bucketName != eid {
				t.Errorf("spec.bucketName mutated: %q, want %q", bucketName, eid)
			}
			storageClassName, _, _ := unstructured.NestedString(obc.Object, "spec", "storageClassName")
			if storageClassName != "rook-ceph-bucket" {
				t.Errorf("spec.storageClassName mutated: %q, want %q", storageClassName, "rook-ceph-bucket")
			}

			additionalConfig, found, _ := unstructured.NestedMap(obc.Object, "spec", "additionalConfig")
			if tt.wantAdditionalConfig == nil {
				if found {
					t.Errorf("spec.additionalConfig should be absent, got %v", additionalConfig)
				}
			} else if !reflect.DeepEqual(additionalConfig, tt.wantAdditionalConfig) {
				t.Errorf("spec.additionalConfig = %v, want %v", additionalConfig, tt.wantAdditionalConfig)
			}
		})
	}
}
