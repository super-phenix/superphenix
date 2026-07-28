package bucket

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	"k8s.io/apimachinery/pkg/api/resource"
)

// defaultMaxMinifiedJSONLen caps policy/lifecycle JSON after minification when
// no limit is configured.
const defaultMaxMinifiedJSONLen = 5 * 1000

// maxMinifiedJSONLen returns the configured cap, or the default when unset.
func maxMinifiedJSONLen() int {
	if v := config.Global.ProductsConfig.S3.MaxMinifiedJSONLen; v > 0 {
		return v
	}
	return defaultMaxMinifiedJSONLen
}

// ValidateAndMinifyJSON returns the compacted JSON or an error if invalid or too long.
func ValidateAndMinifyJSON(field, raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(raw)); err != nil {
		return "", fmt.Errorf("%s is not valid JSON", field)
	}
	if max := maxMinifiedJSONLen(); buf.Len() > max {
		return "", fmt.Errorf("%s exceeds %d characters after minification (%d)", field, max, buf.Len())
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
