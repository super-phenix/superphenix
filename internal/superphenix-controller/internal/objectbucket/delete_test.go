package objectbucket

import (
	"context"
	"testing"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
)

func TestDeleteBucket(t *testing.T) {
	tests := []struct {
		name           string
		labelOverrides map[string]string
		noResource     bool
		wantErr        bool
		wantNotFound   bool
		wantRemaining  bool
	}{
		{
			name: "deletes the bucket",
		},
		{
			name:         "not found",
			noResource:   true,
			wantErr:      true,
			wantNotFound: true,
		},
		{
			name:           "cross-project rejected",
			labelOverrides: map[string]string{spxId.SpxLabelProjectID: "spx-other-project"},
			wantErr:        true,
			wantNotFound:   true,
			wantRemaining:  true,
		},
		{
			name:           "gitops resource rejected",
			labelOverrides: map[string]string{spxId.SpxLabelGitops: "true"},
			wantErr:        true,
			wantRemaining:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setS3Config(t, map[string]string{"standard": "rook-ceph-bucket"}, "", 0, "")

			namespace := testNamespace(t)
			eid := testEffectiveId(t)

			if tt.noResource {
				setFakeDynamicClient(t)
			} else {
				setFakeDynamicClient(t, newOBC(t, namespace, "rook-ceph-bucket", nil, tt.labelOverrides))
			}

			err := DeleteBucket(context.Background(), namespace, eid)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DeleteBucket() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantNotFound && !apierrors.IsNotFound(err) {
				t.Errorf("DeleteBucket() error = %v, want NotFound", err)
			}

			_, err = config.DynamicClientSet.Resource(ObjectBucketClaimGVR).Namespace(namespace).Get(context.Background(), NamePrefix+eid, metav1.GetOptions{})
			remaining := err == nil
			if remaining != tt.wantRemaining && !tt.noResource {
				t.Errorf("OBC remaining = %v, want %v", remaining, tt.wantRemaining)
			}
		})
	}
}
