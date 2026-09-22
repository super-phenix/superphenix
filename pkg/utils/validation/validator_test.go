package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testMacStruct struct {
	MAC string `json:"mac" validate:"omitempty,mac"`
}

func TestMACValidation(t *testing.T) {
	v := GetValidatorV1()

	tests := []struct {
		name        string
		mac         string
		expectError bool
	}{
		{
			name:        "empty MAC passes (omitempty)",
			mac:         "",
			expectError: false,
		},
		{
			name:        "valid colon-delimited MAC passes",
			mac:         "52:54:00:11:22:33",
			expectError: false,
		},
		{
			name:        "hyphen-delimited MAC is rejected",
			mac:         "52-54-00-11-22-33",
			expectError: true,
		},
		{
			name:        "invalid characters rejected",
			mac:         "52:54:00:11:22:zz",
			expectError: true,
		},
		{
			name:        "too short rejected",
			mac:         "52:54:00:11:22",
			expectError: true,
		},
		{
			name:        "arbitrary string rejected",
			mac:         "not-a-mac",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := testMacStruct{MAC: tt.mac}
			err := v.Validator().Struct(s)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Also verify IsValidMAC directly when non-empty
			if tt.mac != "" {
				assert.Equal(t, !tt.expectError, IsValidMAC(tt.mac))
				assert.Equal(t, !tt.expectError, MACRegex.MatchString(tt.mac))
			}
		})
	}
}
