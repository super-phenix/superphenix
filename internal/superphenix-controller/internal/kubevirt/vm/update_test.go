package vm

import (
	"context"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"go.uber.org/mock/gomock"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

func TestUpdateVM_CustomLabels(t *testing.T) {
	const (
		namespace = "prj-test"
		name      = "vm-1"
	)

	baseLabels := map[string]string{
		spxId.SpxLabelProjectID: namespace,
		"some-other-label":      "keep-me",
	}

	newVM := func(extraLabels map[string]string) *v1.VirtualMachine {
		merged := make(map[string]string)
		for k, v := range baseLabels {
			merged[k] = v
		}
		for k, v := range extraLabels {
			merged[k] = v
		}
		return &v1.VirtualMachine{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    merged,
			},
			Spec: v1.VirtualMachineSpec{
				Template: &v1.VirtualMachineInstanceTemplateSpec{
					ObjectMeta: k8smetav1.ObjectMeta{
						Labels: merged,
					},
					Spec: v1.VirtualMachineInstanceSpec{
						Domain: v1.DomainSpec{
							CPU:    &v1.CPU{Cores: 1},
							Memory: &v1.Memory{},
						},
					},
				},
			},
			Status: v1.VirtualMachineStatus{
				PrintableStatus: v1.VirtualMachineStatusStopped,
			},
		}
	}

	// validInfo returns an UpdateVMInfo with valid compute/strategy values so
	// the function progresses past validation into the builder calls (which
	// will fail without full mocks, but that's fine — we only care about labels).
	validInfo := func(labels []string) UpdateVMInfo {
		info := UpdateVMInfo{}
		info.General.RunStrategy = string(v1.RunStrategyAlways)
		info.General.VMType = "linux"
		info.General.Labels = labels
		info.Compute.Cpu = 2
		info.Compute.Memory = 4
		return info
	}

	tests := []struct {
		name           string
		existingLabels map[string]string
		inputLabels    []string
		wantErr        bool
		errContains    string
		wantLabels     map[string]string
	}{
		{
			name:           "add custom labels to VM with none",
			existingLabels: map[string]string{},
			inputLabels:    []string{utils.CustomLabelPrefix + "env:prod"},
			wantLabels: map[string]string{
				spxId.SpxLabelProjectID:         namespace,
				"some-other-label":              "keep-me",
				utils.CustomLabelPrefix + "env": "prod",
			},
		},
		{
			name: "replace old custom labels with new ones",
			existingLabels: map[string]string{
				utils.CustomLabelPrefix + "old-label": "old-value",
			},
			inputLabels: []string{utils.CustomLabelPrefix + "new-label:new-value"},
			wantLabels: map[string]string{
				spxId.SpxLabelProjectID:               namespace,
				"some-other-label":                    "keep-me",
				utils.CustomLabelPrefix + "new-label": "new-value",
			},
		},
		{
			name: "remove all custom labels when input is empty",
			existingLabels: map[string]string{
				utils.CustomLabelPrefix + "env":  "prod",
				utils.CustomLabelPrefix + "team": "backend",
			},
			inputLabels: []string{},
			wantLabels: map[string]string{
				spxId.SpxLabelProjectID: namespace,
				"some-other-label":      "keep-me",
			},
		},
		{
			name:           "non-custom labels are preserved",
			existingLabels: map[string]string{},
			inputLabels:    []string{utils.CustomLabelPrefix + "app:web"},
			wantLabels: map[string]string{
				spxId.SpxLabelProjectID:         namespace,
				"some-other-label":              "keep-me",
				utils.CustomLabelPrefix + "app": "web",
			},
		},
		{
			name:           "invalid label format returns error",
			existingLabels: map[string]string{},
			inputLabels:    []string{"!!!invalid!!!"},
			wantErr:        true,
			errContains:    "parsing custom labels",
		},
		{
			name:           "wrong prefix returns error",
			existingLabels: map[string]string{},
			inputLabels:    []string{"wrong.prefix/key:value"},
			wantErr:        true,
			errContains:    "parsing custom labels",
		},
		{
			name:           "too many custom labels returns error",
			existingLabels: map[string]string{},
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
		{
			name: "multiple custom labels added correctly",
			existingLabels: map[string]string{
				utils.CustomLabelPrefix + "old-label": "willgo",
			},
			inputLabels: []string{
				utils.CustomLabelPrefix + "env:staging",
				utils.CustomLabelPrefix + "team:platform",
			},
			wantLabels: map[string]string{
				spxId.SpxLabelProjectID:          namespace,
				"some-other-label":               "keep-me",
				utils.CustomLabelPrefix + "env":  "staging",
				utils.CustomLabelPrefix + "team": "platform",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			client := kubecli.NewMockKubevirtClient(ctrl)
			vmIface := kubecli.NewMockVirtualMachineInterface(ctrl)

			orig := config.VirtClient
			origK8s := config.K8sClient
			t.Cleanup(func() {
				config.VirtClient = orig
				config.K8sClient = origK8s
			})
			config.VirtClient = client
			config.K8sClient = fake.NewSimpleClientset()

			existingVM := newVM(tt.existingLabels)

			client.EXPECT().VirtualMachine(namespace).Return(vmIface).AnyTimes()
			vmIface.EXPECT().Get(gomock.Any(), name, gomock.Any()).Return(existingVM, nil)
			vmIface.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(existingVM, nil).AnyTimes()

			info := validInfo(tt.inputLabels)

			// UpdateVM may panic on unmocked infrastructure (informers, etc.)
			// after labels are already set. We recover so we can still inspect labels.
			var err error
			func() {
				defer func() { recover() }()
				err = UpdateVM(context.Background(), namespace, name, info)
			}()

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected an error but got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			// Labels are set on the VM object before builder calls, so we can
			// inspect them even if the function panicked on unmocked infrastructure.
			gotLabels := existingVM.GetLabels()
			if len(gotLabels) != len(tt.wantLabels) {
				t.Errorf("label count mismatch: got %d, want %d\ngot:  %v\nwant: %v", len(gotLabels), len(tt.wantLabels), gotLabels, tt.wantLabels)
				return
			}
			for k, wantV := range tt.wantLabels {
				if gotV, ok := gotLabels[k]; !ok {
					t.Errorf("missing expected label %q", k)
				} else if gotV != wantV {
					t.Errorf("label %q = %q, want %q", k, gotV, wantV)
				}
			}

			// Also verify template labels match
			tmplLabels := existingVM.Spec.Template.ObjectMeta.Labels
			for k, wantV := range tt.wantLabels {
				if gotV, ok := tmplLabels[k]; !ok {
					t.Errorf("missing expected template label %q", k)
				} else if gotV != wantV {
					t.Errorf("template label %q = %q, want %q", k, gotV, wantV)
				}
			}
		})
	}
}

func TestCheckDiskOrder(t *testing.T) {
	// makePVCVolume creates a PVC-backed volume (actual disk).
	makePVCVolume := func(name string) v1.Volume {
		return v1.Volume{
			Name: name,
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{},
			},
		}
	}

	// makeCloudInitVolume creates a cloud-init volume (not a disk).
	makeCloudInitVolume := func() v1.Volume {
		return v1.Volume{
			Name: "cloud-init",
			VolumeSource: v1.VolumeSource{
				CloudInitNoCloud: &v1.CloudInitNoCloudSource{},
			},
		}
	}

	// makeContainerDiskVolume creates a container disk volume (not a disk).
	makeContainerDiskVolume := func(name string) v1.Volume {
		return v1.Volume{
			Name: name,
			VolumeSource: v1.VolumeSource{
				ContainerDisk: &v1.ContainerDiskSource{Image: "test-image"},
			},
		}
	}

	makeVMWithVolumes := func(volumes ...v1.Volume) *v1.VirtualMachine {
		return &v1.VirtualMachine{
			Spec: v1.VirtualMachineSpec{
				Template: &v1.VirtualMachineInstanceTemplateSpec{
					Spec: v1.VirtualMachineInstanceSpec{
						Volumes: volumes,
					},
				},
			},
		}
	}

	makeDisks := func(eids ...string) []Disk {
		disks := make([]Disk, len(eids))
		for i, eid := range eids {
			disks[i] = Disk{Order: i, Eid: eid}
		}
		return disks
	}

	tests := []struct {
		name        string
		vm          *v1.VirtualMachine
		diskList    []Disk
		wantErr     bool
		errContains string
	}{
		{
			name:     "new disks added - no error",
			vm:       makeVMWithVolumes(makePVCVolume("disk-a"), makePVCVolume("disk-b")),
			diskList: makeDisks("disk-a", "disk-b", "disk-c"),
			wantErr:  false,
		},
		{
			name:        "disk removed - should error",
			vm:          makeVMWithVolumes(makePVCVolume("disk-a"), makePVCVolume("disk-b"), makePVCVolume("disk-c")),
			diskList:    makeDisks("disk-a", "disk-b"),
			wantErr:     true,
			errContains: "removed",
		},
		{
			name:     "same disks unchanged - no error",
			vm:       makeVMWithVolumes(makePVCVolume("disk-a"), makePVCVolume("disk-b")),
			diskList: makeDisks("disk-a", "disk-b"),
			wantErr:  false,
		},
		{
			name:        "disk order changed - should error",
			vm:          makeVMWithVolumes(makePVCVolume("disk-a"), makePVCVolume("disk-b")),
			diskList:    makeDisks("disk-b", "disk-a", "disk-c"),
			wantErr:     true,
			errContains: "order change",
		},
		{
			name:     "empty VM volumes with new disks - no error",
			vm:       makeVMWithVolumes(),
			diskList: makeDisks("disk-a"),
			wantErr:  false,
		},
		{
			name:     "empty both - no error",
			vm:       makeVMWithVolumes(),
			diskList: makeDisks(),
			wantErr:  false,
		},
		{
			name:     "single volume with matching disk and new disk - no error",
			vm:       makeVMWithVolumes(makePVCVolume("disk-a")),
			diskList: makeDisks("disk-a", "disk-b"),
			wantErr:  false,
		},
		{
			name:        "single volume with mismatched first disk - should error",
			vm:          makeVMWithVolumes(makePVCVolume("disk-a")),
			diskList:    makeDisks("disk-b", "disk-a"),
			wantErr:     true,
			errContains: "order change",
		},
		{
			name:     "cloud-init and container disk volumes are ignored",
			vm:       makeVMWithVolumes(makePVCVolume("disk-a"), makeCloudInitVolume(), makeContainerDiskVolume("cd-1"), makePVCVolume("disk-b")),
			diskList: makeDisks("disk-a", "disk-b"),
			wantErr:  false,
		},
		{
			name:     "only non-disk volumes with new disks - no error",
			vm:       makeVMWithVolumes(makeCloudInitVolume(), makeContainerDiskVolume("cd-1")),
			diskList: makeDisks("disk-a"),
			wantErr:  false,
		},
		{
			name:        "non-disk volumes ignored but disk order changed - should error",
			vm:          makeVMWithVolumes(makeCloudInitVolume(), makePVCVolume("disk-a"), makePVCVolume("disk-b")),
			diskList:    makeDisks("disk-b", "disk-a"),
			wantErr:     true,
			errContains: "order change",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkDiskOrder(tt.vm, tt.diskList)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected an error but got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}
