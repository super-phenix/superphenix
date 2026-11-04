package vm

import (
	"context"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	v1 "kubevirt.io/api/core/v1"
)

const testMemory = "4Gi"

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
		info.Compute.Memory = testMemory
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

func TestConvertToVM_ComputeSize(t *testing.T) {
	metadata := spxId.Metadata{OrgId: "org-1", ProjectId: "prj-test", ResourceEffectiveId: "vm-1"}

	origK8s := config.K8sClient
	t.Cleanup(func() { config.K8sClient = origK8s })
	config.K8sClient = fake.NewSimpleClientset()
	setFakeSubnetWatcher(t)

	idxSSH := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{testNSIndex: cache.MetaNamespaceIndexFunc})
	oldSSH, hadSSH := informers.WatcherSet[informers.SSH]
	informers.WatcherSet[informers.SSH] = informers.Watcher{Indexer: idxSSH}
	t.Cleanup(func() {
		if hadSSH {
			informers.WatcherSet[informers.SSH] = oldSSH
		} else {
			delete(informers.WatcherSet, informers.SSH)
		}
	})

	tests := []struct {
		name        string
		cpu         int
		memory      string
		errContains string
	}{
		{name: "custom size within bounds", cpu: 8, memory: "24Gi"},
		{name: "maximum size", cpu: 999999, memory: "999999Gi"},
		{name: "large size", cpu: 128, memory: "1024Gi"},
		{name: "fractional gibibytes", cpu: 2, memory: "1.5Gi"},
		{name: "mebibytes", cpu: 2, memory: "2500Mi"},
		{name: "cpu above maximum returns error", cpu: 1000000, memory: testMemory, errContains: "cpu value"},
		{name: "cpu zero returns error", cpu: 0, memory: testMemory, errContains: "cpu value"},
		{name: "memory above maximum returns error", cpu: 2, memory: "1000000Gi", errContains: "memory ("},
		{name: "memory below minimum returns error", cpu: 2, memory: "127Mi", errContains: "memory ("},
		{name: "memory unparseable returns error", cpu: 2, memory: "abc", errContains: "memory ("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := CreateVMInfo{}
			info.General.RunStrategy = string(v1.RunStrategyAlways)
			info.Compute.Cpu = tt.cpu
			info.Compute.Memory = tt.memory

			vm, err := convertToVM(context.Background(), "prj-test", info, metadata)
			if tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			domain := vm.Spec.Template.Spec.Domain
			if domain.CPU.Cores != uint32(tt.cpu) {
				t.Errorf("cores = %d, want %d", domain.CPU.Cores, tt.cpu)
			}
			wantMemory := resource.MustParse(tt.memory)
			if domain.Memory.Guest.Cmp(wantMemory) != 0 {
				t.Errorf("guest memory = %s, want %s", domain.Memory.Guest.String(), wantMemory.String())
			}
		})
	}
}
