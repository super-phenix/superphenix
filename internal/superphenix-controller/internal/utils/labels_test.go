package utils

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseLabel(t *testing.T) {
	tests := []struct {
		name    string
		label   string
		wantKey string
		wantVal string
		wantErr bool
	}{
		{
			name:    "valid label",
			label:   "key:value",
			wantKey: "key",
			wantVal: "value",
			wantErr: false,
		},
		{
			name:    "valid label with prefix",
			label:   "example.com/key:value",
			wantKey: "example.com/key",
			wantVal: "value",
			wantErr: false,
		},
		{
			name:    "valid label with dots and dashes",
			label:   "my-app.io/service-name:v1.0.0",
			wantKey: "my-app.io/service-name",
			wantVal: "v1.0.0",
			wantErr: false,
		},
		{
			name:    "empty value is valid",
			label:   "key:",
			wantKey: "key",
			wantVal: "",
			wantErr: false,
		},
		{
			name:    "label too long",
			label:   strings.Repeat("a", 254),
			wantErr: true,
		},
		{
			name:    "invalid format - no colon",
			label:   "keyvalue",
			wantErr: true,
		},
		{
			name:    "invalid format - multiple colons",
			label:   "key:value:extra",
			wantErr: true,
		},
		{
			name:    "invalid format - starts with dash",
			label:   "-key:value",
			wantErr: true,
		},
		{
			name:    "invalid format - invalid characters",
			label:   "key$:value",
			wantErr: true,
		},
		{
			name:    "invalid format - two slashes",
			label:   "example.com/extra/key:value",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, gotVal, err := ParseLabel(tt.label)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLabel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotKey != tt.wantKey {
					t.Errorf("ParseLabel() gotKey = %v, want %v", gotKey, tt.wantKey)
				}
				if gotVal != tt.wantVal {
					t.Errorf("ParseLabel() gotVal = %v, want %v", gotVal, tt.wantVal)
				}
			}
		})
	}
}

func TestParseLabels(t *testing.T) {
	tests := []struct {
		name    string
		labels  []string
		prefix  string
		want    map[string]string
		wantErr bool
	}{
		{
			name:   "valid labels without prefix check",
			labels: []string{"key1:val1", "key2:val2"},
			prefix: "",
			want: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
			wantErr: false,
		},
		{
			name:   "valid labels with matching prefix",
			labels: []string{"spx.io/key1:val1", "spx.io/key2:val2"},
			prefix: "spx.io/",
			want: map[string]string{
				"spx.io/key1": "val1",
				"spx.io/key2": "val2",
			},
			wantErr: false,
		},
		{
			name:    "labels with non-matching prefix",
			labels:  []string{"spx.io/key1:val1", "other.io/key2:val2"},
			prefix:  "spx.io/",
			want:    map[string]string{},
			wantErr: true,
		},
		{
			name:    "contains invalid label",
			labels:  []string{"key1:val1", "invalid-label"},
			prefix:  "",
			want:    map[string]string{},
			wantErr: true,
		},
		{
			name:    "empty labels list",
			labels:  []string{},
			prefix:  "",
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:    "contains label with two slashes",
			labels:  []string{"example.com/extra/key1:val1"},
			prefix:  "",
			want:    map[string]string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLabels(tt.labels, tt.prefix)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLabels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseLabels() got = %v, want %v", got, tt.want)
			}
		})
	}
}
