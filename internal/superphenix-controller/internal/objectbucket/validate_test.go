package objectbucket

import (
	"errors"
	"strings"
	"testing"
)

func TestMinifyJSON(t *testing.T) {
	// whitespace-heavy document that only shrinks once minified
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
		{name: "whitespace-heavy doc is minified", raw: whitespaceHeavy, want: `{"a":"b"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := minifyJSON("policy", tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("minifyJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				var validationErr ValidationError
				if !errors.As(err, &validationErr) {
					t.Errorf("minifyJSON() error is not a ValidationError: %v", err)
				}
				return
			}
			if got != tt.want {
				t.Errorf("minifyJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBucketConfigValidate(t *testing.T) {
	tests := []struct {
		name          string
		maxSizeCap    string
		maxObjectsCap uint64
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
			name:       "valid maxSize",
			maxSizeCap: "1Ti",
			config:     BucketConfig{MaxSize: "10Gi"},
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
			name:       "maxSize over cap",
			maxSizeCap: "1Gi",
			config:     BucketConfig{MaxSize: "10Gi"},
			wantErr:    true,
		},
		{
			name:       "maxSize over cap across units",
			maxSizeCap: "1Gi",
			config:     BucketConfig{MaxSize: "1025Mi"},
			wantErr:    true,
		},
		{
			name:       "maxSize under cap across units",
			maxSizeCap: "1Gi",
			config:     BucketConfig{MaxSize: "1023Mi"},
		},
		{
			name:   "no cap configured",
			config: BucketConfig{MaxSize: "100Ti", MaxObjects: uintPtr(1 << 60)},
		},
		{
			name:          "maxObjects under cap",
			maxObjectsCap: 1000,
			config:        BucketConfig{MaxObjects: uintPtr(1000)},
		},
		{
			name:          "maxObjects over cap",
			maxObjectsCap: 1000,
			config:        BucketConfig{MaxObjects: uintPtr(1001)},
			wantErr:       true,
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
			name:    "invalid lifecycle json",
			config:  BucketConfig{Lifecycle: `not-json`},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setS3Config(t, map[string]string{}, tt.maxSizeCap, tt.maxObjectsCap, "")

			err := tt.config.validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				var validationErr ValidationError
				if !errors.As(err, &validationErr) {
					t.Errorf("validate() error is not a ValidationError: %v", err)
				}
				return
			}
			if tt.config.Policy != tt.wantPolicy {
				t.Errorf("validate() policy = %q, want %q", tt.config.Policy, tt.wantPolicy)
			}
			if tt.config.Lifecycle != tt.wantLifecycle {
				t.Errorf("validate() lifecycle = %q, want %q", tt.config.Lifecycle, tt.wantLifecycle)
			}
		})
	}
}
