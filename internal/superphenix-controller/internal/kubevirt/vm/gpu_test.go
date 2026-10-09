package vm

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"go.uber.org/mock/gomock"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

const (
	testRtxDevice = "nvidia.com/GB202GL_RTX_PRO_6000_BLACKWELL_SERVER_EDITION"
	testA100      = "nvidia.com/GA100_A100_PCIE_40GB"
	testRtxClass  = "nvidia-rtx-pro-6000-bse"
	testA100Class = "nvidia-a100"
	testH100Class = "nvidia-h100"
	testGpuName   = "gpu-0"
	testVMType    = "linux"
	testNamespace = "prj-test"
	testNSIndex   = "namespace"
)

func setDeviceMapping(t *testing.T, entries map[string]config.GpuClassEntry) {
	t.Helper()
	prev := config.Global.ProductsConfig.Compute.DeviceMapping
	config.Global.ProductsConfig.Compute.DeviceMapping = entries
	t.Cleanup(func() { config.Global.ProductsConfig.Compute.DeviceMapping = prev })
}

var sampleDeviceMapping = map[string]config.GpuClassEntry{
	testRtxClass:  {ID: testRtxClass, DisplayName: "NVIDIA RTX PRO 6000", DeviceName: testRtxDevice},
	testA100Class: {ID: testA100Class, DisplayName: "NVIDIA A100", DeviceName: testA100},
}

func TestResolveGPUs(t *testing.T) {
	setDeviceMapping(t, sampleDeviceMapping)

	tests := []struct {
		name        string
		gpus        []GPU
		want        []v1.GPU
		errContains string
	}{
		{name: "nil means no gpu", gpus: nil, want: []v1.GPU{}},
		{name: "empty means no gpu", gpus: []GPU{}, want: []v1.GPU{}},
		{
			name: "one gpu is resolved and named gpu-0",
			gpus: []GPU{{Device: testRtxClass}},
			want: []v1.GPU{{Name: testGpuName, DeviceName: testRtxDevice}},
		},
		{name: "unknown class is rejected", gpus: []GPU{{Device: testH100Class}}, errContains: `unknown class "` + testH100Class + `"`},
		{name: "empty class is rejected", gpus: []GPU{{Device: ""}}, errContains: "empty class"},
		{
			name:        "more than one gpu is rejected",
			gpus:        []GPU{{Device: testRtxClass}, {Device: testA100Class}},
			errContains: "at most 1 allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveGPUs(tt.gpus)
			if tt.errContains != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q, got nil", tt.errContains)
				}
				if !strings.HasPrefix(err.Error(), errInvalidGpuPrefix) {
					t.Errorf("error %q does not start with %q", err.Error(), errInvalidGpuPrefix)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("resolveGPUs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSetGPUs(t *testing.T) {
	existing := []v1.GPU{{Name: testGpuName, DeviceName: testA100}}
	rtx := []v1.GPU{{Name: testGpuName, DeviceName: testRtxDevice}}

	tests := []struct {
		name      string
		cpu       *v1.CPU
		gpus      []v1.GPU
		want      []v1.GPU
		wantModel string
	}{
		{name: "empty list removes gpus and the host-passthrough model", cpu: &v1.CPU{Cores: 2, Model: v1.CPUModeHostPassthrough}, gpus: []v1.GPU{}, want: nil, wantModel: ""},
		{name: "nil list removes gpus and the host-passthrough model", cpu: &v1.CPU{Cores: 2, Model: v1.CPUModeHostPassthrough}, gpus: nil, want: nil, wantModel: ""},
		{name: "removing gpus keeps another cpu model", cpu: &v1.CPU{Cores: 2, Model: "Skylake-Server"}, gpus: nil, want: nil, wantModel: "Skylake-Server"},
		{name: "list replaces gpus and sets host-passthrough", cpu: &v1.CPU{Cores: 2}, gpus: rtx, want: rtx, wantModel: v1.CPUModeHostPassthrough},
		{name: "adding gpus without cpu block creates it", cpu: nil, gpus: rtx, want: rtx, wantModel: v1.CPUModeHostPassthrough},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &v1.VirtualMachine{Spec: v1.VirtualMachineSpec{Template: &v1.VirtualMachineInstanceTemplateSpec{}}}
			vm.Spec.Template.Spec.Domain.Devices.GPUs = existing
			vm.Spec.Template.Spec.Domain.CPU = tt.cpu

			setGPUs(vm, tt.gpus)

			if got := vm.Spec.Template.Spec.Domain.Devices.GPUs; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GPUs = %#v, want %#v", got, tt.want)
			}
			gotModel := ""
			if cpu := vm.Spec.Template.Spec.Domain.CPU; cpu != nil {
				gotModel = cpu.Model
			}
			if gotModel != tt.wantModel {
				t.Errorf("CPU model = %q, want %q", gotModel, tt.wantModel)
			}
		})
	}
}

func TestGpuClassList(t *testing.T) {
	tests := []struct {
		name    string
		mapping map[string]config.GpuClassEntry
		wantIDs []string
	}{
		{name: "nil mapping returns an empty list", mapping: nil, wantIDs: []string{}},
		{name: "entries are sorted by id", mapping: sampleDeviceMapping, wantIDs: []string{testA100Class, testRtxClass}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDeviceMapping(t, tt.mapping)

			got := GpuClassList()
			if got == nil {
				t.Fatal("GpuClassList() returned nil, want a non-nil slice")
			}
			ids := make([]string, 0, len(got))
			for _, entry := range got {
				ids = append(ids, entry.ID)
			}
			if !reflect.DeepEqual(ids, tt.wantIDs) {
				t.Errorf("ids = %v, want %v", ids, tt.wantIDs)
			}
		})
	}
}

// TestConvertToVM_InvalidGpus checks GPUs are validated before any resource is built.
func TestConvertToVM_InvalidGpus(t *testing.T) {
	setDeviceMapping(t, sampleDeviceMapping)

	metadata := spxId.Metadata{OrgId: "org-1", ProjectId: testNamespace, ResourceEffectiveId: "vm-1"}

	tests := []struct {
		name string
		gpus []GPU
	}{
		{name: "unknown class", gpus: []GPU{{Device: testH100Class}}},
		{name: "too many gpus", gpus: []GPU{{Device: testA100Class}, {Device: testA100Class}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := CreateVMInfo{}
			info.General.RunStrategy = string(v1.RunStrategyAlways)
			info.General.VMType = testVMType
			info.Compute.Cpu = 2
			info.Compute.Memory = 4
			info.Compute.Gpu = tt.gpus

			_, err := convertToVM(context.Background(), testNamespace, info, metadata)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.HasPrefix(err.Error(), errInvalidGpuPrefix) {
				t.Errorf("error %q does not start with %q", err.Error(), errInvalidGpuPrefix)
			}
		})
	}
}

func TestUpdateVM_Gpus(t *testing.T) {
	const (
		namespace = testNamespace
		name      = "vm-1"
	)
	setDeviceMapping(t, sampleDeviceMapping)
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

	existingGpus := []v1.GPU{{Name: testGpuName, DeviceName: testA100}}

	tests := []struct {
		name        string
		gpu         *[]GPU
		want        []v1.GPU
		wantErr     bool
		wantUpdated bool
		wantModel   string
	}{
		{name: "nil preserves the current gpus and cpu model", gpu: nil, want: existingGpus, wantUpdated: true, wantModel: v1.CPUModeHostPassthrough},
		{name: "empty list removes the gpus and host-passthrough", gpu: &[]GPU{}, want: nil, wantUpdated: true, wantModel: ""},
		{
			name:        "list replaces the gpus",
			gpu:         &[]GPU{{Device: testRtxClass}},
			want:        []v1.GPU{{Name: testGpuName, DeviceName: testRtxDevice}},
			wantUpdated: true,
			wantModel:   v1.CPUModeHostPassthrough,
		},
		{name: "unknown class is rejected without updating", gpu: &[]GPU{{Device: testH100Class}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			client := kubecli.NewMockKubevirtClient(ctrl)
			vmIface := kubecli.NewMockVirtualMachineInterface(ctrl)

			origVirt := config.VirtClient
			origK8s := config.K8sClient
			t.Cleanup(func() {
				config.VirtClient = origVirt
				config.K8sClient = origK8s
			})
			config.VirtClient = client
			config.K8sClient = fake.NewSimpleClientset()

			existingVM := &v1.VirtualMachine{
				ObjectMeta: k8smetav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels:    map[string]string{spxId.SpxLabelProjectID: namespace},
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						ObjectMeta: k8smetav1.ObjectMeta{
							Labels: map[string]string{spxId.SpxLabelProjectID: namespace},
						},
						Spec: v1.VirtualMachineInstanceSpec{
							Domain: v1.DomainSpec{
								CPU:     &v1.CPU{Cores: 1, Model: v1.CPUModeHostPassthrough},
								Memory:  &v1.Memory{},
								Devices: v1.Devices{GPUs: existingGpus},
							},
						},
					},
				},
				Status: v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusStopped},
			}

			var updatedVM *v1.VirtualMachine
			client.EXPECT().VirtualMachine(namespace).Return(vmIface).AnyTimes()
			vmIface.EXPECT().Get(gomock.Any(), name, gomock.Any()).Return(existingVM, nil).AnyTimes()
			vmIface.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, vm *v1.VirtualMachine, opts k8smetav1.UpdateOptions) (*v1.VirtualMachine, error) {
					updatedVM = vm
					return vm, nil
				},
			).AnyTimes()

			info := UpdateVMInfo{}
			info.General.RunStrategy = string(v1.RunStrategyAlways)
			info.General.VMType = testVMType
			info.Compute.Cpu = 2
			info.Compute.Memory = 4
			info.Compute.Gpu = tt.gpu

			err := UpdateVM(context.Background(), namespace, name, info)
			if tt.wantErr {
				if err == nil || !strings.HasPrefix(err.Error(), errInvalidGpuPrefix) {
					t.Fatalf("expected an %q error, got %v", errInvalidGpuPrefix, err)
				}
				if updatedVM != nil {
					t.Fatal("VM was updated although the request was rejected")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error updating VM: %v", err)
			}
			if updatedVM == nil {
				t.Fatal("expected VM to be updated via client, but Update was not called")
			}
			if got := updatedVM.Spec.Template.Spec.Domain.CPU.Model; got != tt.wantModel {
				t.Errorf("CPU model = %q, want %q", got, tt.wantModel)
			}
			if got := updatedVM.Spec.Template.Spec.Domain.Devices.GPUs; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GPUs = %#v, want %#v", got, tt.want)
			}
		})
	}
}
