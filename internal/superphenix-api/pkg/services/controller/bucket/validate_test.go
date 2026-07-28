package bucket

import (
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	"github.com/stretchr/testify/assert"
)

// setMaxMinifiedJSONLen overrides the configured cap
func setMaxMinifiedJSONLen(t *testing.T, max int) {
	t.Helper()
	old := config.Global.S3.MaxMinifiedJSONLen
	config.Global.S3.MaxMinifiedJSONLen = max
	t.Cleanup(func() { config.Global.S3.MaxMinifiedJSONLen = old })
}

func TestValidateAndMinifyJSON(t *testing.T) {
	max := maxMinifiedJSONLen()
	// {"a":"..."} weighs 8 chars plus the value length
	exactlyMax := `{"a":"` + strings.Repeat("x", max-8) + `"}`
	overMax := `{"a":"` + strings.Repeat("x", max-7) + `"}`
	// whitespace-heavy document that only fits once minified
	whitespaceHeavy := `{` + strings.Repeat(" ", 2*max) + `"a":  "b"  }`

	tests := []struct {
		name          string
		raw           string
		configuredMax int
		want          string
		wantErr       bool
	}{
		{name: "empty", raw: "", want: ""},
		{name: "pretty json is compacted", raw: "{\n  \"a\": \"b\"\n}", want: `{"a":"b"}`},
		{name: "invalid json", raw: `{"a":`, wantErr: true},
		{name: "exactly max chars", raw: exactlyMax, want: exactlyMax},
		{name: "over max chars", raw: overMax, wantErr: true},
		{name: "fits only after minification", raw: whitespaceHeavy, want: `{"a":"b"}`},
		{name: "configured cap wins", raw: `{"a":"bcdefghijklmnop"}`, configuredMax: 10, wantErr: true},
		{name: "zero config falls back to default", raw: exactlyMax, configuredMax: -1, want: exactlyMax},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.configuredMax != 0 {
				setMaxMinifiedJSONLen(t, tt.configuredMax)
			}
			got, err := ValidateAndMinifyJSON("policy", tt.raw)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateBucketConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        BucketConfig
		wantErr       bool
		wantPolicy    string
		wantLifecycle string
	}{
		{
			name:   "empty config",
			config: BucketConfig{},
		},
		{
			name:   "nil maxObjects",
			config: BucketConfig{MaxObjects: nil, MaxSize: "10Gi"},
		},
		{
			name:    "invalid maxSize",
			config:  BucketConfig{MaxSize: "10XB"},
			wantErr: true,
		},
		{
			name:    "negative maxSize",
			config:  BucketConfig{MaxSize: "-5Gi"},
			wantErr: true,
		},
		{
			name:          "policy and lifecycle minified in place",
			config:        BucketConfig{Policy: "{\n  \"Version\": \"2012-10-17\"\n}", Lifecycle: "{ \"Rules\" : [] }"},
			wantPolicy:    `{"Version":"2012-10-17"}`,
			wantLifecycle: `{"Rules":[]}`,
		},
		{
			name:       "only policy set leaves lifecycle untouched",
			config:     BucketConfig{Policy: `{ "a": 1 }`},
			wantPolicy: `{"a":1}`,
		},
		{
			name:    "invalid policy json",
			config:  BucketConfig{Policy: `not-json`},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBucketConfig(&tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantPolicy, tt.config.Policy)
			assert.Equal(t, tt.wantLifecycle, tt.config.Lifecycle)
		})
	}
}
