package vm

import (
	"context"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "kubevirt.io/api/core/v1"
)

func TestConvertToVM_CustomLabels(t *testing.T) {
	const namespace = "prj-test"

	metadata := spxId.Metadata{
		OrgId:               "org-1",
		ProjectId:           namespace,
		ResourceEffectiveId: "vm-1",
	}

	baseInfo := func(labels []string) CreateVMInfo {
		info := CreateVMInfo{}
		info.General.RunStrategy = string(v1.RunStrategyAlways)
		info.General.VMType = "linux"
		info.General.Labels = labels
		info.Compute.Cpu = 2
		info.Compute.Memory = 4
		return info
	}

	tests := []struct {
		name        string
		inputLabels []string
		wantErr     bool
		errContains string
	}{
		{
			name:        "invalid label format returns error",
			inputLabels: []string{"!!!invalid!!!"},
			wantErr:     true,
			errContains: "parsing custom labels",
		},
		{
			name:        "wrong prefix returns error",
			inputLabels: []string{"wrong.prefix/key:value"},
			wantErr:     true,
			errContains: "parsing custom labels",
		},
		{
			name: "too many custom labels returns error",
			inputLabels: []string{
				utils.CustomLabelPrefix + "l1:v1",
				utils.CustomLabelPrefix + "l2:v2",
				utils.CustomLabelPrefix + "l3:v3",
				utils.CustomLabelPrefix + "l4:v4",
				utils.CustomLabelPrefix + "l5:v5",
				utils.CustomLabelPrefix + "l6:v6",
				utils.CustomLabelPrefix + "l7:v7",
				utils.CustomLabelPrefix + "l8:v8",
				utils.CustomLabelPrefix + "l9:v9",
				utils.CustomLabelPrefix + "l10:v10",
				utils.CustomLabelPrefix + "l11:v11",
			},
			wantErr:     true,
			errContains: "too many custom labels",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := baseInfo(tt.inputLabels)
			_, err := convertToVM(context.Background(), namespace, info, metadata)

			if !tt.wantErr {
				t.Fatalf("all test cases in this table should expect errors")
			}
			if err == nil {
				t.Errorf("expected an error but got nil")
			} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
			}
		})
	}
}
