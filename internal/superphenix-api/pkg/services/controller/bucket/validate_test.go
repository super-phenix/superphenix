package bucket

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateAndMinifyJSON(t *testing.T) {
	// {"a":"..."} weighs 8 chars plus the value length
	exactly1000 := `{"a":"` + strings.Repeat("x", 992) + `"}`
	over1000 := `{"a":"` + strings.Repeat("x", 993) + `"}`
	// whitespace-heavy document that only fits once minified
	whitespaceHeavy := `{` + strings.Repeat(" ", 2000) + `"a":  "b"  }`

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "empty", raw: "", want: ""},
		{name: "pretty json is compacted", raw: "{\n  \"a\": \"b\"\n}", want: `{"a":"b"}`},
		{name: "invalid json", raw: `{"a":`, wantErr: true},
		{name: "exactly 1000 chars", raw: exactly1000, want: exactly1000},
		{name: "over 1000 chars", raw: over1000, wantErr: true},
		{name: "fits only after minification", raw: whitespaceHeavy, want: `{"a":"b"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
