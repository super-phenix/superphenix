package objectbucket

import (
	"strconv"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

// BucketConfig maps to the OBC spec.additionalConfig block.
type BucketConfig struct {
	MaxObjects *uint64 `json:"maxObjects,omitempty"`
	MaxSize    string  `json:"maxSize,omitempty"`   // kubernetes quantity, e.g. "100Gi"
	Policy     string  `json:"policy,omitempty"`    // S3 bucket policy JSON
	Lifecycle  string  `json:"lifecycle,omitempty"` // S3 lifecycle configuration JSON
}

type CreateBucketInfo struct {
	spxId.Metadata
	General struct {
		StorageClass string       `json:"storageClass"`
		Config       BucketConfig `json:"config"`
	} `json:"general"`
}

type UpdateBucketInfo struct {
	General struct {
		Config BucketConfig `json:"config"`
	} `json:"general"`
}

// additionalConfig returns the OBC spec.additionalConfig content. All values
// are strings, only set fields are present, matching the GitOps chart shape.
func (c *BucketConfig) additionalConfig() map[string]interface{} {
	m := map[string]interface{}{}
	if c.MaxObjects != nil {
		m["bucketMaxObjects"] = strconv.FormatUint(*c.MaxObjects, 10)
	}
	if c.MaxSize != "" {
		m["bucketMaxSize"] = c.MaxSize
	}
	if c.Policy != "" {
		m["bucketPolicy"] = c.Policy
	}
	if c.Lifecycle != "" {
		m["bucketLifecycle"] = c.Lifecycle
	}
	return m
}
