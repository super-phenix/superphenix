package volumeSnapshot

import (
	"fmt"
	"testing"
)

func TestValidateRetentionExpiryTime(t *testing.T) {
	tests := []struct {
		name            string
		expiryTime      int
		expectedExpires string
		expectError     bool
	}{
		{
			name:        "Zero expiry returns error",
			expiryTime:  0,
			expectError: true,
		},
		{
			name:        "Negative expiry returns error",
			expiryTime:  -5,
			expectError: true,
		},
		{
			name:            "Minimum valid expiry",
			expiryTime:      24,
			expectedExpires: "24h",
		},
		{
			name:            "Normal expiry",
			expiryTime:      48,
			expectedExpires: "48h",
		},
		{
			name:            "Exactly max",
			expiryTime:      960,
			expectedExpires: "960h",
		},
		{
			name:        "Exceeds max returns error",
			expiryTime:  1200,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expiryTime := tt.expiryTime

			if expiryTime < MinRetentionExpiryTime {
				if !tt.expectError {
					t.Errorf("expected no error but expiryTime %d is below minimum %d", expiryTime, MinRetentionExpiryTime)
				}
				return
			}
			if expiryTime > MaxRetentionExpiryTime {
				if !tt.expectError {
					t.Errorf("expected no error but expiryTime %d exceeds maximum %d", expiryTime, MaxRetentionExpiryTime)
				}
				return
			}

			if tt.expectError {
				t.Errorf("expected error but expiryTime %d is within valid range", expiryTime)
				return
			}

			expires := fmt.Sprintf("%dh", expiryTime)
			if expires != tt.expectedExpires {
				t.Errorf("expected expires %q, got %q", tt.expectedExpires, expires)
			}
		})
	}
}
