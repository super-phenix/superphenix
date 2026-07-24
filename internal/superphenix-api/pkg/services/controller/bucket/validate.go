package bucket

import (
	"bytes"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

// maxMinifiedJSONLen caps the size of policy/lifecycle JSON after minification.
// The AZ controller enforces the same rule; this check owns the user-facing error.
const maxMinifiedJSONLen = 1000

// ValidateAndMinifyJSON returns the compacted JSON or an error if invalid or too long.
func ValidateAndMinifyJSON(field, raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(raw)); err != nil {
		return "", fmt.Errorf("%s is not valid JSON", field)
	}
	if buf.Len() > maxMinifiedJSONLen {
		return "", fmt.Errorf("%s exceeds %d characters after minification (%d)", field, maxMinifiedJSONLen, buf.Len())
	}
	return buf.String(), nil
}

// ValidateBucketConfig minifies policy/lifecycle in place and syntax-checks
// maxSize. The per-AZ caps are enforced by the AZ controller, which owns them.
func ValidateBucketConfig(c *BucketConfig) error {
	if c.MaxSize != "" {
		size, err := resource.ParseQuantity(c.MaxSize)
		if err != nil {
			return fmt.Errorf("maxSize is not a valid quantity")
		}
		if size.Sign() <= 0 {
			return fmt.Errorf("maxSize must be positive")
		}
	}

	policy, err := ValidateAndMinifyJSON("policy", c.Policy)
	if err != nil {
		return err
	}
	c.Policy = policy

	lifecycle, err := ValidateAndMinifyJSON("lifecycle", c.Lifecycle)
	if err != nil {
		return err
	}
	c.Lifecycle = lifecycle

	return nil
}
