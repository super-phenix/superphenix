package volumeSnapshot

import (
	"testing"
)

func TestSnapshotScheduleConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      SnapshotScheduleConfig
		expectError bool
	}{
		{
			name:        "Valid range 0-23",
			config:      SnapshotScheduleConfig{MinHour: 0, MaxHour: 23},
			expectError: false,
		},
		{
			name:        "Valid same hour",
			config:      SnapshotScheduleConfig{MinHour: 10, MaxHour: 10},
			expectError: false,
		},
		{
			name:        "Valid normal range",
			config:      SnapshotScheduleConfig{MinHour: 2, MaxHour: 6},
			expectError: false,
		},
		{
			name:        "MinHour negative",
			config:      SnapshotScheduleConfig{MinHour: -1, MaxHour: 10},
			expectError: true,
		},
		{
			name:        "MaxHour negative",
			config:      SnapshotScheduleConfig{MinHour: 0, MaxHour: -1},
			expectError: true,
		},
		{
			name:        "MinHour exceeds 23",
			config:      SnapshotScheduleConfig{MinHour: 24, MaxHour: 23},
			expectError: true,
		},
		{
			name:        "MaxHour exceeds 23",
			config:      SnapshotScheduleConfig{MinHour: 0, MaxHour: 24},
			expectError: true,
		},
		{
			name:        "MinHour greater than MaxHour",
			config:      SnapshotScheduleConfig{MinHour: 15, MaxHour: 10},
			expectError: true,
		},
		{
			name:        "Both negative",
			config:      SnapshotScheduleConfig{MinHour: -5, MaxHour: -1},
			expectError: true,
		},
		{
			name:        "Both exceed 23",
			config:      SnapshotScheduleConfig{MinHour: 25, MaxHour: 30},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}
