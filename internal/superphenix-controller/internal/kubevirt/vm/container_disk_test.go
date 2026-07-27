package vm

import (
	"slices"
	"strings"
	"testing"

	v1 "kubevirt.io/api/core/v1"
)

func newEmptyVM() *v1.VirtualMachine {
	return &v1.VirtualMachine{
		Spec: v1.VirtualMachineSpec{
			Template: &v1.VirtualMachineInstanceTemplateSpec{},
		},
	}
}

func cdVolume(name, image string) v1.Volume {
	return v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			ContainerDisk: &v1.ContainerDiskSource{Image: image},
		},
	}
}

func TestValidateMountSpec(t *testing.T) {
	tests := []struct {
		name          string
		spec          ContainerDiskSpec
		wantErrSubstr string
	}{
		{name: "valid sata", spec: ContainerDiskSpec{Image: "img", Bus: "sata", ID: "windows-virtio-drivers"}},
		{name: "valid virtio", spec: ContainerDiskSpec{Image: "img", Bus: "virtio", ID: "foo"}},
		{name: "empty image", spec: ContainerDiskSpec{Image: "", Bus: "sata", ID: "v"}, wantErrSubstr: "image is empty"},
		{name: "empty volume name", spec: ContainerDiskSpec{Image: "img", Bus: "sata", ID: ""}, wantErrSubstr: "id is empty"},
		{name: "invalid bus", spec: ContainerDiskSpec{Image: "img", Bus: "scsi", ID: "v"}, wantErrSubstr: "bus"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMountSpec(tt.spec)
			if tt.wantErrSubstr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErrSubstr) {
				t.Fatalf("err = %v, want substring %q", err, tt.wantErrSubstr)
			}
		})
	}
}

func TestValidateUnmountSpec(t *testing.T) {
	tests := []struct {
		name          string
		spec          ContainerDiskSpec
		wantErrSubstr string
	}{
		{name: "volume name only is enough", spec: ContainerDiskSpec{ID: "windows-virtio-drivers"}},
		{name: "full spec accepted (extra fields ignored)", spec: ContainerDiskSpec{Image: "img", Bus: "sata", ID: "v"}},
		{name: "missing volume name rejected", spec: ContainerDiskSpec{Image: "img"}, wantErrSubstr: "id is empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUnmountSpec(tt.spec)
			if tt.wantErrSubstr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErrSubstr) {
				t.Fatalf("err = %v, want substring %q", err, tt.wantErrSubstr)
			}
		})
	}
}

func TestShouldSkipMount(t *testing.T) {
	tests := []struct {
		name    string
		volumes []v1.Volume
		spec    ContainerDiskSpec
		want    bool
	}{
		{
			name:    "exact name match → skip",
			volumes: []v1.Volume{cdVolume("windows-virtio-drivers", "img")},
			spec:    ContainerDiskSpec{Image: "img", Bus: "sata", ID: "windows-virtio-drivers"},
			want:    true,
		},
		{
			name:    "empty VM → don't skip",
			volumes: nil,
			spec:    ContainerDiskSpec{Image: "img", Bus: "sata", ID: "windows-virtio-drivers"},
			want:    false,
		},
		{
			name:    "unrelated volume → don't skip",
			volumes: []v1.Volume{{Name: "rootfs"}},
			spec:    ContainerDiskSpec{Image: "img", Bus: "sata", ID: "windows-virtio-drivers"},
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := newEmptyVM()
			vm.Spec.Template.Spec.Volumes = tt.volumes
			if got := shouldSkipMount(vm, tt.spec); got != tt.want {
				t.Errorf("shouldSkipMount = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyUnmount(t *testing.T) {
	tests := []struct {
		name        string
		volumes     []v1.Volume
		disks       []v1.Disk
		spec        ContainerDiskSpec
		wantRemoved bool
		wantVolumes []string
		wantDisks   []string
	}{
		{
			name:        "bug repro: VM with windows-virtio-drivers, unmount with same volumeName → removed",
			volumes:     []v1.Volume{cdVolume("windows-virtio-drivers", "img")},
			disks:       []v1.Disk{{Name: "windows-virtio-drivers"}},
			spec:        ContainerDiskSpec{ID: "windows-virtio-drivers"},
			wantRemoved: true,
			wantVolumes: []string{},
			wantDisks:   []string{},
		},
		{
			name:        "empty VM → no-op",
			volumes:     nil,
			disks:       nil,
			spec:        ContainerDiskSpec{ID: "windows-virtio-drivers"},
			wantRemoved: false,
		},
		{
			name: "VM with target plus unrelated volumes → only target removed",
			volumes: []v1.Volume{
				{Name: "rootfs"},
				cdVolume("windows-virtio-drivers", "img"),
				{Name: "cloud-init"},
			},
			disks: []v1.Disk{
				{Name: "rootfs"},
				{Name: "windows-virtio-drivers"},
				{Name: "cloud-init"},
			},
			spec:        ContainerDiskSpec{ID: "windows-virtio-drivers"},
			wantRemoved: true,
			wantVolumes: []string{"rootfs", "cloud-init"},
			wantDisks:   []string{"rootfs", "cloud-init"},
		},
		{
			name:        "unrelated volume only → no-op",
			volumes:     []v1.Volume{{Name: "rootfs"}},
			disks:       []v1.Disk{{Name: "rootfs"}},
			spec:        ContainerDiskSpec{ID: "windows-virtio-drivers"},
			wantRemoved: false,
		},
		{
			name:        "empty spec.ID matches nothing",
			volumes:     []v1.Volume{cdVolume("windows-virtio-drivers", "img")},
			disks:       []v1.Disk{{Name: "windows-virtio-drivers"}},
			spec:        ContainerDiskSpec{ID: ""},
			wantRemoved: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := newEmptyVM()
			vm.Spec.Template.Spec.Volumes = tt.volumes
			vm.Spec.Template.Spec.Domain.Devices.Disks = tt.disks

			got := applyUnmount(vm, tt.spec)
			if got != tt.wantRemoved {
				t.Fatalf("applyUnmount = %v, want %v", got, tt.wantRemoved)
			}
			if !tt.wantRemoved {
				return
			}
			gotVolumeNames := make([]string, 0, len(vm.Spec.Template.Spec.Volumes))
			for _, v := range vm.Spec.Template.Spec.Volumes {
				gotVolumeNames = append(gotVolumeNames, v.Name)
			}
			gotDiskNames := make([]string, 0, len(vm.Spec.Template.Spec.Domain.Devices.Disks))
			for _, d := range vm.Spec.Template.Spec.Domain.Devices.Disks {
				gotDiskNames = append(gotDiskNames, d.Name)
			}
			if !slices.Equal(gotVolumeNames, tt.wantVolumes) {
				t.Errorf("volumes = %v, want %v", gotVolumeNames, tt.wantVolumes)
			}
			if !slices.Equal(gotDiskNames, tt.wantDisks) {
				t.Errorf("disks = %v, want %v", gotDiskNames, tt.wantDisks)
			}
		})
	}
}

func TestSnapshotContainerDisks(t *testing.T) {
	dvVolume := func(name string) v1.Volume {
		return v1.Volume{
			Name: name,
			VolumeSource: v1.VolumeSource{
				DataVolume: &v1.DataVolumeSource{Name: "dv-" + name},
			},
		}
	}

	tests := []struct {
		name            string
		volumes         []v1.Volume
		disks           []v1.Disk
		wantVolumeNames []string
		wantDiskNames   []string
	}{
		{
			name:            "no container disks",
			volumes:         []v1.Volume{dvVolume("rootfs")},
			disks:           []v1.Disk{{Name: "rootfs"}},
			wantVolumeNames: []string{},
			wantDiskNames:   []string{},
		},
		{
			name: "container disks preserved along with their disks",
			volumes: []v1.Volume{
				dvVolume("rootfs"),
				cdVolume("windows-virtio-drivers", "img"),
			},
			disks: []v1.Disk{
				{Name: "rootfs"},
				{Name: "windows-virtio-drivers"},
			},
			wantVolumeNames: []string{"windows-virtio-drivers"},
			wantDiskNames:   []string{"windows-virtio-drivers"},
		},
		{
			name: "DataVolume named like a container disk is not preserved",
			volumes: []v1.Volume{
				dvVolume("windows-virtio-drivers"),
			},
			disks: []v1.Disk{
				{Name: "windows-virtio-drivers"},
			},
			wantVolumeNames: []string{},
			wantDiskNames:   []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vols, disks := snapshotContainerDisks(tt.volumes, tt.disks)
			gotVolNames := make([]string, 0, len(vols))
			for _, v := range vols {
				gotVolNames = append(gotVolNames, v.Name)
			}
			gotDiskNames := make([]string, 0, len(disks))
			for _, d := range disks {
				gotDiskNames = append(gotDiskNames, d.Name)
			}
			if !slices.Equal(gotVolNames, tt.wantVolumeNames) {
				t.Errorf("volumes = %v, want %v", gotVolNames, tt.wantVolumeNames)
			}
			if !slices.Equal(gotDiskNames, tt.wantDiskNames) {
				t.Errorf("disks = %v, want %v", gotDiskNames, tt.wantDiskNames)
			}
		})
	}
}

func TestApplyMounts(t *testing.T) {
	tests := []struct {
		name           string
		existing       []v1.Volume
		specs          []ContainerDiskSpec
		wantChanged    bool
		wantVolumes    []string
		wantBuses      []v1.DiskBus
		wantErrSubstr  string
		wantNoMutation bool // when validation fails, the VM must be untouched
	}{
		{
			name:        "empty batch is a no-op",
			specs:       []ContainerDiskSpec{},
			wantChanged: false,
			wantVolumes: []string{},
		},
		{
			name: "all new disks attach in input order with mapped buses",
			specs: []ContainerDiskSpec{
				{ID: "a", Image: "img-a", Bus: "sata"},
				{ID: "b", Image: "img-b", Bus: "virtio"},
			},
			wantChanged: true,
			wantVolumes: []string{"a", "b"},
			wantBuses:   []v1.DiskBus{v1.DiskBusSATA, v1.DiskBusVirtio},
		},
		{
			name:     "all already mounted → no-op, changed=false",
			existing: []v1.Volume{cdVolume("a", "img-a"), cdVolume("b", "img-b")},
			specs: []ContainerDiskSpec{
				{ID: "a", Image: "img-a", Bus: "sata"},
				{ID: "b", Image: "img-b", Bus: "virtio"},
			},
			wantChanged: false,
			wantVolumes: []string{"a", "b"},
		},
		{
			name:     "mix: existing skipped, new attached",
			existing: []v1.Volume{cdVolume("a", "img-a")},
			specs: []ContainerDiskSpec{
				{ID: "a", Image: "img-a", Bus: "sata"},
				{ID: "b", Image: "img-b", Bus: "virtio"},
			},
			wantChanged: true,
			wantVolumes: []string{"a", "b"},
		},
		{
			name: "duplicate IDs in batch → error, no mutation",
			specs: []ContainerDiskSpec{
				{ID: "a", Image: "img", Bus: "sata"},
				{ID: "a", Image: "img", Bus: "sata"},
			},
			wantErrSubstr:  "duplicate id",
			wantNoMutation: true,
		},
		{
			name: "invalid spec mid-batch → error, no partial application",
			specs: []ContainerDiskSpec{
				{ID: "a", Image: "img", Bus: "sata"},
				{ID: "b", Image: "", Bus: "sata"},
				{ID: "c", Image: "img", Bus: "sata"},
			},
			wantErrSubstr:  "image is empty",
			wantNoMutation: true,
		},
		{
			name: "invalid bus rejected",
			specs: []ContainerDiskSpec{
				{ID: "a", Image: "img", Bus: "scsi"},
			},
			wantErrSubstr:  "bus",
			wantNoMutation: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := newEmptyVM()
			vm.Spec.Template.Spec.Volumes = tt.existing

			changed, err := applyMounts(vm, tt.specs)

			gotVolumeNames := make([]string, 0, len(vm.Spec.Template.Spec.Volumes))
			for _, v := range vm.Spec.Template.Spec.Volumes {
				gotVolumeNames = append(gotVolumeNames, v.Name)
			}

			if tt.wantErrSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Fatalf("err = %v, want substring %q", err, tt.wantErrSubstr)
				}
				if tt.wantNoMutation {
					existingNames := make([]string, 0, len(tt.existing))
					for _, v := range tt.existing {
						existingNames = append(existingNames, v.Name)
					}
					if !slices.Equal(gotVolumeNames, existingNames) {
						t.Errorf("VM was mutated despite validation error: volumes = %v, want unchanged %v", gotVolumeNames, existingNames)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if !slices.Equal(gotVolumeNames, tt.wantVolumes) {
				t.Errorf("volumes = %v, want %v", gotVolumeNames, tt.wantVolumes)
			}
			if tt.wantBuses != nil {
				disks := vm.Spec.Template.Spec.Domain.Devices.Disks
				if len(disks) != len(tt.wantBuses) {
					t.Fatalf("disks len = %d, want %d", len(disks), len(tt.wantBuses))
				}
				for i, d := range disks {
					if d.DiskDevice.CDRom == nil {
						t.Fatalf("disk %d is not a CDROM", i)
					}
					if d.DiskDevice.CDRom.Bus != tt.wantBuses[i] {
						t.Errorf("disk %d bus = %q, want %q", i, d.DiskDevice.CDRom.Bus, tt.wantBuses[i])
					}
				}
			}
		})
	}
}

func TestApplyUnmounts(t *testing.T) {
	tests := []struct {
		name          string
		volumes       []v1.Volume
		disks         []v1.Disk
		specs         []ContainerDiskSpec
		wantChanged   bool
		wantVolumes   []string
		wantErrSubstr string
	}{
		{
			name:        "empty batch is a no-op",
			volumes:     []v1.Volume{cdVolume("a", "img")},
			disks:       []v1.Disk{{Name: "a"}},
			specs:       []ContainerDiskSpec{},
			wantChanged: false,
			wantVolumes: []string{"a"},
		},
		{
			name:    "all present → all removed, changed=true",
			volumes: []v1.Volume{cdVolume("a", "img-a"), cdVolume("b", "img-b")},
			disks:   []v1.Disk{{Name: "a"}, {Name: "b"}},
			specs: []ContainerDiskSpec{
				{ID: "a"},
				{ID: "b"},
			},
			wantChanged: true,
			wantVolumes: []string{},
		},
		{
			name:    "all absent → no-op, changed=false",
			volumes: []v1.Volume{{Name: "rootfs"}},
			disks:   []v1.Disk{{Name: "rootfs"}},
			specs: []ContainerDiskSpec{
				{ID: "a"},
				{ID: "b"},
			},
			wantChanged: false,
			wantVolumes: []string{"rootfs"},
		},
		{
			name:    "mix: present removed, absent skipped",
			volumes: []v1.Volume{{Name: "rootfs"}, cdVolume("a", "img-a")},
			disks:   []v1.Disk{{Name: "rootfs"}, {Name: "a"}},
			specs: []ContainerDiskSpec{
				{ID: "a"},
				{ID: "not-mounted"},
			},
			wantChanged: true,
			wantVolumes: []string{"rootfs"},
		},
		{
			name:          "empty ID in batch rejected",
			volumes:       []v1.Volume{cdVolume("a", "img")},
			disks:         []v1.Disk{{Name: "a"}},
			specs:         []ContainerDiskSpec{{ID: ""}},
			wantErrSubstr: "id is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := newEmptyVM()
			vm.Spec.Template.Spec.Volumes = tt.volumes
			vm.Spec.Template.Spec.Domain.Devices.Disks = tt.disks

			changed, err := applyUnmounts(vm, tt.specs)
			if tt.wantErrSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Fatalf("err = %v, want substring %q", err, tt.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
			gotVolumeNames := make([]string, 0, len(vm.Spec.Template.Spec.Volumes))
			for _, v := range vm.Spec.Template.Spec.Volumes {
				gotVolumeNames = append(gotVolumeNames, v.Name)
			}
			if !slices.Equal(gotVolumeNames, tt.wantVolumes) {
				t.Errorf("volumes = %v, want %v", gotVolumeNames, tt.wantVolumes)
			}
		})
	}
}

