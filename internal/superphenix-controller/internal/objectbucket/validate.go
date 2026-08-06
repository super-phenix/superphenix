package objectbucket

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	"k8s.io/apimachinery/pkg/api/resource"
)

// ValidationError marks user errors so handlers can answer 400 instead of 500.
type ValidationError struct {
	msg string
}

func (e ValidationError) Error() string {
	return e.msg
}

func newValidationError(format string, args ...interface{}) ValidationError {
	return ValidationError{msg: fmt.Sprintf(format, args...)}
}

// resolveStorageClass maps a friendly class name to the real OBC storage class.
// An empty mapping means S3 is not available in this AZ.
func resolveStorageClass(friendly string) (string, error) {
	fullname, ok := config.Global.ProductsConfig.ObjectStorage.StorageClassMapping[friendly]
	if !ok || fullname == "" {
		return "", newValidationError("no storage class found")
	}
	return fullname, nil
}

// minifyJSON compacts a JSON document
func minifyJSON(field, raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(raw)); err != nil {
		return "", newValidationError("%s is not valid JSON", field)
	}
	return buf.String(), nil
}

// validate minifies policy/lifecycle in place and enforces the AZ caps.
func (c *BucketConfig) validate() error {
	if c.MaxSize != "" {
		size, err := resource.ParseQuantity(c.MaxSize)
		if err != nil {
			return newValidationError("maxSize is not a valid quantity")
		}
		if size.Sign() <= 0 {
			return newValidationError("maxSize must be positive")
		}
		if capStr := config.Global.ProductsConfig.ObjectStorage.MaxBucketSize; capStr != "" {
			maxSize, err := resource.ParseQuantity(capStr)
			if err != nil {
				return fmt.Errorf("invalid s3.maxBucketSize in configuration: %w", err)
			}
			if size.Cmp(maxSize) > 0 {
				return newValidationError("maxSize exceeds the maximum allowed in this availability zone (%s)", capStr)
			}
		}
	}

	if c.MaxObjects != nil {
		if maxObjects := config.Global.ProductsConfig.ObjectStorage.MaxBucketObjects; maxObjects > 0 && *c.MaxObjects > maxObjects {
			return newValidationError("maxObjects exceeds the maximum allowed in this availability zone (%d)", maxObjects)
		}
	}

	policy, err := minifyJSON("policy", c.Policy)
	if err != nil {
		return err
	}
	c.Policy = policy

	lifecycle, err := minifyJSON("lifecycle", c.Lifecycle)
	if err != nil {
		return err
	}
	c.Lifecycle = lifecycle

	return nil
}
