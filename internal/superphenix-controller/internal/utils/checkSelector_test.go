package utils

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCheckSelector(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid selector",
			key:     "example.com/name",
			value:   "my-value",
			wantErr: false,
		},
		{
			name:    "empty value is valid",
			key:     "my-key",
			value:   "",
			wantErr: false,
		},
		{
			name:    "key too long",
			key:     strings.Repeat("a", 253-63+1),
			value:   "val",
			wantErr: true,
			errMsg:  "selector key too long",
		},
		{
			name:    "value too long",
			key:     "key",
			value:   strings.Repeat("a", 64),
			wantErr: true,
			errMsg:  "selector value too long",
		},
		{
			name:    "empty key",
			key:     "",
			value:   "val",
			wantErr: true,
			errMsg:  "selector key is empty",
		},
		{
			name:    "invalid key format",
			key:     "_invalid",
			value:   "val",
			wantErr: true,
			errMsg:  "invalid selector key",
		},
		{
			name:    "invalid value format",
			key:     "key",
			value:   "_invalid",
			wantErr: true,
			errMsg:  "invalid selector value",
		},
		{
			name:    "key with slash and dots",
			key:     "example.com/path.to_key",
			value:   "val",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckSelector(tt.key, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckSelector() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("CheckSelector() error = %v, wantErrMsg %v", err, tt.errMsg)
			}
		})
	}
}

func TestCheckExpressions(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		operator string
		values   []string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid expression In",
			key:      "tier",
			operator: string(metav1.LabelSelectorOpIn),
			values:   []string{"frontend", "backend"},
			wantErr:  false,
		},
		{
			name:     "valid expression Exists",
			key:      "tier",
			operator: string(metav1.LabelSelectorOpExists),
			values:   nil,
			wantErr:  false,
		},
		{
			name:     "valid expression NotIn",
			key:      "tier",
			operator: string(metav1.LabelSelectorOpNotIn),
			values:   []string{"frontend"},
			wantErr:  false,
		},
		{
			name:     "valid expression DoesNotExist",
			key:      "tier",
			operator: string(metav1.LabelSelectorOpDoesNotExist),
			values:   nil,
			wantErr:  false,
		},
		{
			name:     "key too long",
			key:      strings.Repeat("a", 253-63+1),
			operator: string(metav1.LabelSelectorOpIn),
			values:   []string{"val"},
			wantErr:  true,
			errMsg:   "selector key too long",
		},
		{
			name:     "empty key",
			key:      "",
			operator: string(metav1.LabelSelectorOpIn),
			values:   []string{"val"},
			wantErr:  true,
			errMsg:   "selector key is empty",
		},
		{
			name:     "invalid operator",
			key:      "key",
			operator: "Invalid",
			values:   []string{"val"},
			wantErr:  true,
			errMsg:   "invalid operator",
		},
		{
			name:     "value too long in expressions",
			key:      "key",
			operator: string(metav1.LabelSelectorOpIn),
			values:   []string{"valid", strings.Repeat("a", 64)},
			wantErr:  true,
			errMsg:   "selector value too long",
		},
		{
			name:     "invalid value in expressions",
			key:      "key",
			operator: string(metav1.LabelSelectorOpIn),
			values:   []string{"-invalid"},
			wantErr:  true,
			errMsg:   "invalid selector value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckExpressions(tt.key, tt.operator, tt.values)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckExpressions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("CheckExpressions() error = %v, wantErrMsg %v", err, tt.errMsg)
			}
		})
	}
}
