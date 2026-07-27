package datavolume

import (
	"reflect"
	"testing"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func TestCreateDiskInfo_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		disk CreateDiskInfo
		want bool
	}{
		{
			name: "zero value is empty",
			disk: CreateDiskInfo{},
			want: true,
		},
		{
			name: "with OrgId is not empty",
			disk: func() CreateDiskInfo {
				d := CreateDiskInfo{}
				d.OrgId = "some-org"
				return d
			}(),
			want: false,
		},
		{
			name: "with ProjectId is not empty",
			disk: func() CreateDiskInfo {
				d := CreateDiskInfo{}
				d.ProjectId = "some-project"
				return d
			}(),
			want: false,
		},
		{
			name: "with Storage is not empty",
			disk: func() CreateDiskInfo {
				d := CreateDiskInfo{}
				d.General.Storage = "10Gi"
				return d
			}(),
			want: false,
		},
		{
			name: "with Source type is not empty",
			disk: func() CreateDiskInfo {
				d := CreateDiskInfo{}
				d.General.Source.Type = SourceTypeBlank
				return d
			}(),
			want: false,
		},
		{
			name: "with StorageClass is not empty",
			disk: func() CreateDiskInfo {
				d := CreateDiskInfo{}
				d.General.StorageClass = "local-path"
				return d
			}(),
			want: false,
		},
		{
			name: "with Labels is not empty",
			disk: func() CreateDiskInfo {
				d := CreateDiskInfo{}
				d.General.Labels = []string{"label1"}
				return d
			}(),
			want: false,
		},
		{
			name: "fully populated is not empty",
			disk: func() CreateDiskInfo {
				d := CreateDiskInfo{
					Metadata: spxId.Metadata{
						OrgId:     "org-id",
						ProjectId: "project-id",
					},
				}
				d.General.Storage = "20Gi"
				d.General.Source = Source{Type: SourceTypeRegistry, URL: "docker://image"}
				d.General.StorageClass = "ceph"
				d.General.Labels = []string{"a", "b"}
				return d
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.disk.IsEmpty(); got != tt.want {
				t.Errorf("CreateDiskInfo.IsEmpty() = %v, want %v", got, tt.want)
			}

			// This check is used to cover change in the struct
			if got := reflect.DeepEqual(tt.disk, CreateDiskInfo{}); got != tt.want {
				t.Errorf("reflect.DeepEqual(tt.disk, CreateDiskInfo{}) = %v, want %v", got, tt.want)
			}
		})
	}
}
