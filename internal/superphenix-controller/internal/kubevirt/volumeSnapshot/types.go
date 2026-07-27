package volumeSnapshot

import (
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

type RetentionPolicy struct {
	ExpiryTime int `json:"expiryTime,omitempty"`
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
}

type UpdateSnapshotInfo struct {
	Spec struct {
		Schedule      int             `json:"schedule"`
		Paused        bool            `json:"paused"`
		LabelSelector []string        `json:"labelSelector,omitempty"`
		Retention     RetentionPolicy `json:"retention"`
	} `json:"spec"`
}
