package volumeSnapshot

import (
	"fmt"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

type RetentionPolicy struct {
	ExpiryTime int `json:"expiryTime,omitempty"`
}

const (
	MinSnapshotScheduleHour = 0
	MaxSnapshotScheduleHour = 23
)

type SnapshotScheduleConfig struct {
	MinHour int `json:"minHour"`
	MaxHour int `json:"maxHour"`
}

func (s *SnapshotScheduleConfig) Validate() error {
	if s.MinHour < MinSnapshotScheduleHour || s.MinHour > MaxSnapshotScheduleHour {
		return fmt.Errorf("minHour must be between %d and %d", MinSnapshotScheduleHour, MaxSnapshotScheduleHour)
	}
	if s.MaxHour < MinSnapshotScheduleHour || s.MaxHour > MaxSnapshotScheduleHour {
		return fmt.Errorf("maxHour must be between %d and %d", MinSnapshotScheduleHour, MaxSnapshotScheduleHour)
	}
	if s.MinHour > s.MaxHour {
		return fmt.Errorf("minHour (%d) must be less than or equal to maxHour (%d)", s.MinHour, s.MaxHour)
	}
	return nil
}

type CreateSnapshotInfo struct {
	spxId.Metadata
	Spec struct {
		Source        string          `json:"source"`
		Scheduled     bool            `json:"scheduled,omitempty"`
		Schedule      int             `json:"schedule,omitempty"`
		Paused        bool            `json:"paused,omitempty"`
		LabelSelector []string        `json:"labelSelector,omitempty"`
		Retention     RetentionPolicy `json:"retention,omitempty"`
	} `json:"spec"`
	SnapshotSchedule SnapshotScheduleConfig `json:"snapshotSchedule"`
}

type UpdateSnapshotInfo struct {
	Spec struct {
		Schedule      int             `json:"schedule"`
		Paused        bool            `json:"paused"`
		LabelSelector []string        `json:"labelSelector,omitempty"`
		Retention     RetentionPolicy `json:"retention"`
	} `json:"spec"`
	SnapshotSchedule SnapshotScheduleConfig `json:"snapshotSchedule"`
}
