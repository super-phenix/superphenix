package objectbucket

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// ObjectBucketClaimGVR is declared literally: the objectbucket.io types
	// (lib-bucket-provisioner) are not vendored, all access goes through the
	// dynamic client.
	ObjectBucketClaimGVR = schema.GroupVersionResource{
		Group:    "objectbucket.io",
		Version:  "v1alpha1",
		Resource: "objectbucketclaims",
	}

	// ObjectBucketGVR is the OB bound to an OBC. It carries the S3 endpoint
	// (spec.endpoint) and is cluster-scoped, so reads use no namespace.
	ObjectBucketGVR = schema.GroupVersionResource{
		Group:    "objectbucket.io",
		Version:  "v1alpha1",
		Resource: "objectbuckets",
	}

	// CephObjectStoreGVR is the Rook store an OB belongs to (namespaced). The
	// OB endpoint carries no scheme, only the store knows whether it is TLS.
	CephObjectStoreGVR = schema.GroupVersionResource{
		Group:    "ceph.rook.io",
		Version:  "v1",
		Resource: "cephobjectstores",
	}
)

// NamePrefix prefixes the OBC name. The provisioner creates a ConfigMap and a
// Secret named after the OBC; without a prefix they would collide with other
// resources named directly after the effective ID.
const NamePrefix = "bucket-"
