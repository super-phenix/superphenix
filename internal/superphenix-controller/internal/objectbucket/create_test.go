package objectbucket

import (
	"context"
	"errors"
	"reflect"
	"testing"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
)

func TestCreateBucket(t *testing.T) {
	mapping := map[string]string{"standard": "rook-ceph-bucket"}

	tests := []struct {
		name                 string
		mapping              map[string]string
		maxSizeCap           string
		maxObjectsCap        uint64
		storageClass         string
		config               BucketConfig
		existing             bool
		wantErr              bool
		wantValidationErr    bool
		wantAdditionalConfig map[string]interface{}
	}{
		{
			name:         "full config",
			mapping:      mapping,
			storageClass: "standard",
			config: BucketConfig{
				MaxObjects: uintPtr(100),
				MaxSize:    "10Gi",
				Policy:     "{\n  \"Version\": \"2012-10-17\"\n}",
				Lifecycle:  `{ "Rules": [] }`,
			},
			wantAdditionalConfig: map[string]interface{}{
				"bucketMaxObjects": "100",
				"bucketMaxSize":    "10Gi",
				"bucketPolicy":     `{"Version":"2012-10-17"}`,
				"bucketLifecycle":  `{"Rules":[]}`,
			},
		},
		{
			name:         "no advanced config",
			mapping:      mapping,
			storageClass: "standard",
			config:       BucketConfig{},
		},
		{
			name:              "unknown storage class",
			mapping:           mapping,
			storageClass:      "premium",
			wantErr:           true,
			wantValidationErr: true,
		},
		{
			name:              "empty mapping disables the feature",
			mapping:           map[string]string{},
			storageClass:      "standard",
			wantErr:           true,
			wantValidationErr: true,
		},
		{
			name:              "maxSize over AZ cap",
			mapping:           mapping,
			maxSizeCap:        "1Gi",
			storageClass:      "standard",
			config:            BucketConfig{MaxSize: "10Gi"},
			wantErr:           true,
			wantValidationErr: true,
		},
		{
			name:              "maxObjects over AZ cap",
			mapping:           mapping,
			maxObjectsCap:     10,
			storageClass:      "standard",
			config:            BucketConfig{MaxObjects: uintPtr(100)},
			wantErr:           true,
			wantValidationErr: true,
		},
		{
			name:              "invalid policy json",
			mapping:           mapping,
			storageClass:      "standard",
			config:            BucketConfig{Policy: `{"a":`},
			wantErr:           true,
			wantValidationErr: true,
		},
		{
			name:         "already exists",
			mapping:      mapping,
			storageClass: "standard",
			existing:     true,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setS3Config(t, tt.mapping, tt.maxSizeCap, tt.maxObjectsCap, "")

			namespace := testNamespace(t)
			var existing []runtime.Object
			if tt.existing {
				existing = append(existing, newOBC(t, namespace, "rook-ceph-bucket", nil, nil))
			}
			setFakeDynamicClient(t, existing...)

			info := CreateBucketInfo{Metadata: testMetadata(t)}
			info.General.StorageClass = tt.storageClass
			info.General.Config = tt.config

			err := info.CreateBucket(context.Background(), namespace)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CreateBucket() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				var validationErr ValidationError
				if errors.As(err, &validationErr) != tt.wantValidationErr {
					t.Errorf("CreateBucket() validation error = %v, want %v", !tt.wantValidationErr, tt.wantValidationErr)
				}
				return
			}

			m := testMetadata(t)
			eid := m.GetResourceEffectiveID()

			obc, err := config.DynamicClientSet.Resource(ObjectBucketClaimGVR).Namespace(namespace).Get(context.Background(), NamePrefix+eid, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("created OBC not found: %v", err)
			}

			bucketName, _, _ := unstructured.NestedString(obc.Object, "spec", "bucketName")
			if bucketName != eid {
				t.Errorf("spec.bucketName = %q, want %q", bucketName, eid)
			}

			storageClassName, _, _ := unstructured.NestedString(obc.Object, "spec", "storageClassName")
			if storageClassName != "rook-ceph-bucket" {
				t.Errorf("spec.storageClassName = %q, want %q", storageClassName, "rook-ceph-bucket")
			}

			labels := obc.GetLabels()
			for key, want := range map[string]string{
				spxId.SpxLabelOrganizationID:      m.GetOrgID(),
				spxId.SpxLabelProjectID:           m.GetProjectID(),
				spxId.SpxLabelResourceLocalID:     testLocalId,
				spxId.SpxLabelResourceEffectiveID: eid,
			} {
				if labels[key] != want {
					t.Errorf("label %s = %q, want %q", key, labels[key], want)
				}
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
