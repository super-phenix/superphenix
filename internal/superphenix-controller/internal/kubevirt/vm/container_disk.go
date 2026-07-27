package vm

import (
	"context"
	"fmt"

	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/api/core/v1"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
)

// ContainerDiskSpec is one mounted disk. The ID is the catalog id and the
// volume name on the VM spec.
type ContainerDiskSpec struct {
	ID    string `json:"id"`
	Image string `json:"image"`
	Bus   string `json:"bus"`
}

// ContainerDiskBatch carries every id in one call so the VM is updated once,
// avoiding write conflicts between back-to-back per-disk calls.
type ContainerDiskBatch struct {
	Ids []string `json:"ids"`
}

// snapshotContainerDisks returns the subset of volumes and disks whose source
// is a ContainerDisk, so they can be restored after a wipe.
func snapshotContainerDisks(volumes []v1.Volume, disks []v1.Disk) ([]v1.Volume, []v1.Disk) {
	preservedVolumes := make([]v1.Volume, 0)
	preservedNames := make(map[string]struct{})
	for _, vol := range volumes {
		if vol.VolumeSource.ContainerDisk != nil {
			preservedVolumes = append(preservedVolumes, vol)
			preservedNames[vol.Name] = struct{}{}
		}
	}
	preservedDisks := make([]v1.Disk, 0)
	for _, d := range disks {
		if _, ok := preservedNames[d.Name]; ok {
			preservedDisks = append(preservedDisks, d)
		}
	}
	return preservedVolumes, preservedDisks
}

func validateMountSpec(s ContainerDiskSpec) error {
	if s.Image == "" {
		return fmt.Errorf("invalid container disk spec: image is empty")
	}
	if s.ID == "" {
		return fmt.Errorf("invalid container disk spec: id is empty")
	}
	if s.Bus != "sata" && s.Bus != "virtio" {
		return fmt.Errorf("invalid container disk spec: bus %q must be sata or virtio", s.Bus)
	}
	return nil
}

func validateUnmountSpec(s ContainerDiskSpec) error {
	if s.ID == "" {
		return fmt.Errorf("invalid container disk spec: id is empty")
	}
	return nil
}

func shouldSkipMount(vm *v1.VirtualMachine, spec ContainerDiskSpec) bool {
	for _, vol := range vm.Spec.Template.Spec.Volumes {
		if vol.Name == spec.ID {
			return true
		}
	}
	return false
}

// applyUnmount removes the Volume named spec.ID and the matching Disk
// entry. Returns whether anything was removed.
func applyUnmount(vm *v1.VirtualMachine, spec ContainerDiskSpec) bool {
	volumes := vm.Spec.Template.Spec.Volumes
	newVolumes := make([]v1.Volume, 0, len(volumes))
	removed := false
	for _, vol := range volumes {
		if vol.Name == spec.ID {
			removed = true
			continue
		}
		newVolumes = append(newVolumes, vol)
	}
	if !removed {
		return false
	}
	vm.Spec.Template.Spec.Volumes = newVolumes

	disks := vm.Spec.Template.Spec.Domain.Devices.Disks
	newDisks := make([]v1.Disk, 0, len(disks))
	for _, d := range disks {
		if d.Name == spec.ID {
			continue
		}
		newDisks = append(newDisks, d)
	}
	vm.Spec.Template.Spec.Domain.Devices.Disks = newDisks
	return true
}

func attachContainerDisk(vm *v1.VirtualMachine, spec ContainerDiskSpec) {
	vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, v1.Volume{
		Name: spec.ID,
		VolumeSource: v1.VolumeSource{
			ContainerDisk: &v1.ContainerDiskSource{Image: spec.Image},
		},
	})

	bus := v1.DiskBusSATA
	if spec.Bus == "virtio" {
		bus = v1.DiskBusVirtio
	}
	ro := true
	vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, v1.Disk{
		Name: spec.ID,
		DiskDevice: v1.DiskDevice{
			CDRom: &v1.CDRomTarget{ReadOnly: &ro, Bus: bus},
		},
	})
}

// applyMounts validates the batch, then attaches each spec not already on the
// VM. Nothing is mutated on error. Returns whether anything changed.
func applyMounts(vm *v1.VirtualMachine, specs []ContainerDiskSpec) (bool, error) {
	seen := make(map[string]struct{}, len(specs))
	for _, s := range specs {
		if err := validateMountSpec(s); err != nil {
			return false, err
		}
		if _, dup := seen[s.ID]; dup {
			return false, fmt.Errorf("invalid container disk batch: duplicate id %q", s.ID)
		}
		seen[s.ID] = struct{}{}
	}
	changed := false
	for _, s := range specs {
		if shouldSkipMount(vm, s) {
			continue
		}
		attachContainerDisk(vm, s)
		changed = true
	}
	return changed, nil
}

// applyUnmounts validates the whole batch first, then removes every spec
// that's actually mounted. Missing entries are silently skipped.
func applyUnmounts(vm *v1.VirtualMachine, specs []ContainerDiskSpec) (bool, error) {
	for _, s := range specs {
		if err := validateUnmountSpec(s); err != nil {
			return false, err
		}
	}
	changed := false
	for _, s := range specs {
		if applyUnmount(vm, s) {
			changed = true
		}
	}
	return changed, nil
}

// MountContainerDisks attaches the given disks in a single VM update.
// Already-mounted disks are skipped; the update is elided if nothing changes.
func MountContainerDisks(ctx context.Context, namespace, vmName string, specs []ContainerDiskSpec) error {
	log := logger.GetLogger(ctx)
	if len(specs) == 0 {
		return nil
	}

	vm, err := config.VirtClient.VirtualMachine(namespace).Get(ctx, vmName, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", vmName).Msg("Failed to find the VM")
		return err
	}

	changed, err := applyMounts(vm, specs)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	if _, err := config.VirtClient.VirtualMachine(namespace).Update(ctx, vm, k8smetav1.UpdateOptions{}); err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", vmName).Int("count", len(specs)).Msg("Failed to mount container disks")
		return err
	}
	return nil
}

// UnmountContainerDisks removes the given disks in a single VM update.
// Missing disks are skipped; the update is elided if nothing changes.
func UnmountContainerDisks(ctx context.Context, namespace, vmName string, specs []ContainerDiskSpec) error {
	log := logger.GetLogger(ctx)
	if len(specs) == 0 {
		return nil
	}

	vm, err := config.VirtClient.VirtualMachine(namespace).Get(ctx, vmName, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", vmName).Msg("Failed to find the VM")
		return err
	}

	changed, err := applyUnmounts(vm, specs)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	if _, err := config.VirtClient.VirtualMachine(namespace).Update(ctx, vm, k8smetav1.UpdateOptions{}); err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", vmName).Int("count", len(specs)).Msg("Failed to unmount container disks")
		return err
	}
	return nil
}
