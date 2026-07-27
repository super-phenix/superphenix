package datavolume

import (
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

const (
	SourceTypeRegistry = "registry"
	SourceTypeHttp     = "http"
	SourceTypeClone    = "clone"
	SourceTypeSnapshot = "snapshot"
	SourceTypeBlank    = "blank"
)

type CreateDiskInfo struct {
	spxId.Metadata
	General struct {
		Storage      string   `json:"storage"` // Storage Size
		Source       Source   `json:"source"`
		StorageClass string   `json:"storageClass"`
		Labels       []string `json:"labels,omitempty"`
	} `json:"general"`
}

// IsEmpty reports whether c is a zero-value CreateDiskInfo without using
// reflect.DeepEqual, which is significantly slower.
func (info *CreateDiskInfo) IsEmpty() bool {
	return info.OrgId == "" &&
		info.ProjectId == "" &&
		info.ResourceEffectiveId == "" &&
		info.ResourceLocalId == "" &&
		info.General.Storage == "" &&
		info.General.Source == (Source{}) &&
		info.General.StorageClass == "" &&
		len(info.General.Labels) == 0
}

type Source struct {
	Type     string `json:"type"`
	URL      string `json:"url,omitempty"`
	Clone    string `json:"clone,omitempty"`
	Snapshot string `json:"snapshot,omitempty"`
}
