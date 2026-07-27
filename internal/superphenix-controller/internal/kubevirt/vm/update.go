package vm

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"k8s.io/apimachinery/pkg/api/resource"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/api/core/v1"
)

func UpdateVM(ctx context.Context, namespace, name string, vmInfo UpdateVMInfo) error {
	log := logger.GetLogger(ctx)
	// Update the VM
	vmToUpdate, err := config.VirtClient.VirtualMachine(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to find the VM")
		return err
	}

	if err := utils.CheckProjectLabel(vmToUpdate, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("VM access denied")
		return err
	}

	if err := utils.IsEditAllowed(vmToUpdate.GetLabels()); err != nil {
		log.Error().Err(err).Any("info", vmInfo).Str("namespace", namespace).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	labels := vmToUpdate.GetLabels()
	customLabels, err := utils.ParseLabels(vmInfo.General.Labels, utils.CustomLabelPrefix)
	if err != nil {
		log.Error().Err(err).Any("info", vmInfo).Str("namespace", namespace).Str("name", name).Msg("Cannot parse custom labels")
		return fmt.Errorf("error parsing custom labels: %w", err)
	}
	if len(customLabels) > utils.MaxCustomLabelNumber {
		log.Error().Any("info", vmInfo).Str("namespace", namespace).Str("name", name).Msgf("Too many custom labels %d > %d", len(customLabels), utils.MaxCustomLabelNumber)
		return fmt.Errorf("too many custom labels %d > %d", len(customLabels), utils.MaxCustomLabelNumber)
	}

	// Remove old custom labels
	maps.DeleteFunc(labels, func(k string, v string) bool {
		return strings.HasPrefix(k, utils.CustomLabelPrefix)
	})
	// Add custom labels
	maps.Copy(labels, customLabels)

	vmToUpdate.Labels = labels
	vmToUpdate.Spec.Template.ObjectMeta.Labels = labels

	// Set global param
	//Validation part
	if !slices.Contains(MEMORY_VALUE_LIST, vmInfo.Compute.Memory) {
		return fmt.Errorf("no such memory: %d", vmInfo.Compute.Memory)
	}
	if !slices.Contains(CPU_VALUE_LIST, vmInfo.Compute.Cpu) {
		return fmt.Errorf("no such cpu value: %d", vmInfo.Compute.Cpu)
	}

	if !slices.Contains(RUN_STRATEGY_VALUE_LIST, vmInfo.General.RunStrategy) {
		return fmt.Errorf("unknown run strategy: %s", vmInfo.General.RunStrategy)
	}

	vmPreference := "linux"
	if vmInfo.General.VMType != "" {
		vmPreference = vmInfo.General.VMType

	}

	// Create correct values format
	runStrategy := v1.VirtualMachineRunStrategy(vmInfo.General.RunStrategy)
	memory, err := resource.ParseQuantity(fmt.Sprintf("%dGi", vmInfo.Compute.Memory))
	cores := uint32(vmInfo.Compute.Cpu)

	vmToUpdate.Spec.RunStrategy = &runStrategy
	vmToUpdate.Spec.Template.Spec.Domain.CPU.Cores = cores
	vmToUpdate.Spec.Template.Spec.Domain.Memory.Guest = &memory

	if vmToUpdate.Spec.Preference != nil {
		vmToUpdate.Spec.Preference.Name = vmPreference
	} else {
		vmToUpdate.Spec.Preference = &v1.PreferenceMatcher{
			Name: vmPreference,
		}
	}

	if vmToUpdate.Status.PrintableStatus != v1.VirtualMachineStatusStopped {
		log.Info().Msg("vm not stopped - checking disk order")
		if err := checkDiskOrder(vmToUpdate, vmInfo.Disks); err != nil {
			log.Err(err).Any("disks", vmInfo.Disks).Msg("Failed to add disks")
			return err
		}
	}

	// Snapshot full container-disk volume+disk pairs before wiping so a nil
	// ContainerDisks field can restore them verbatim.
	preservedVolumes, preservedDisks := snapshotContainerDisks(vmToUpdate.Spec.Template.Spec.Volumes, vmToUpdate.Spec.Template.Spec.Domain.Devices.Disks)

	// Clean old data
	vmToUpdate.Spec.Template.Spec.Volumes = make([]v1.Volume, 0)
	vmToUpdate.Spec.Template.Spec.Domain.Devices.Disks = make([]v1.Disk, 0)
	vmToUpdate.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	vmToUpdate.Spec.Template.Spec.Domain.Devices.Interfaces = make([]v1.Interface, 0)
	vmToUpdate.Spec.Template.Spec.Networks = make([]v1.Network, 0)

	// Do the update
	if err := withDataVolumeDisks(ctx, vmToUpdate, vmInfo.Disks); err != nil {
		log.Err(err).Any("disks", vmInfo.Disks).Msg("Failed to add disks")
		return err
	}

	if err := withCloudInit(ctx, vmToUpdate, vmInfo.CloudInit); err != nil {
		log.Err(err).Msg("Failed to add cloud init")
		return err
	}

	if vmInfo.ContainerDisks == nil {
		vmToUpdate.Spec.Template.Spec.Volumes = append(vmToUpdate.Spec.Template.Spec.Volumes, preservedVolumes...)
		vmToUpdate.Spec.Template.Spec.Domain.Devices.Disks = append(vmToUpdate.Spec.Template.Spec.Domain.Devices.Disks, preservedDisks...)
	} else {
		specs, err := Resolve(*vmInfo.ContainerDisks, vmInfo.General.VMType)
		if err != nil {
			log.Err(err).Msg("Failed to resolve container disks")
			return err
		}
		if _, err := applyMounts(vmToUpdate, specs); err != nil {
			log.Err(err).Msg("Failed to apply container disks")
			return err
		}
	}

	if err := withNetworks(ctx, namespace, vmInfo.Network, vmToUpdate); err != nil {
		log.Err(err).Str("namespace", namespace).Any("vmInfo", vmInfo).Msg("Failed to add networks")
		return err
	}

	if err := withSSHKeys(ctx, namespace, vmInfo.SSHKeys, vmToUpdate); err != nil {
		log.Err(err).Str("namespace", namespace).Any("vmInfo", vmInfo).Msg("Failed to add ssh keys")
		return err
	}

	withAdvancedOptions(vmToUpdate, vmInfo.Advanced)

	// Update the VM
	_, err = config.VirtClient.VirtualMachine(vmToUpdate.Namespace).Update(ctx, vmToUpdate, k8smetav1.UpdateOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Any("vmInfo", vmInfo).Msg("Failed to update VM")
		return err
	}

	// Due to a bug in Kubevirt, labels are not propapi-gatewayed if the VM is running
	// To compensate, we manually add user custom labels directly to the virt-launcher pod
	if vmToUpdate.Status.PrintableStatus == v1.VirtualMachineStatusRunning {
		pod, err := GetVirtLauncherPod(ctx, vmToUpdate.Namespace, vmToUpdate.Name)
		if err != nil {
			log.Err(err).Str("namespace", namespace).Any("vmInfo", vmInfo).Msg("Failed to fetch the virtual launcher")
		}

		if err := UpdateVirtLauncherPodLabels(ctx, pod, customLabels); err != nil {
			log.Err(err).Str("namespace", namespace).Any("podName", pod.Name).Msg("Failed to update the virtual launcher")
		}
	}

	return nil

}

func UnmountDisk(ctx context.Context, namespace, name, diskEID string) error {
	log := logger.GetLogger(ctx)
	vm, err := config.VirtClient.VirtualMachine(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Any("name", name).Msg("Failed to find the VM")
		return err
	}

	if vm.Status.PrintableStatus != v1.VirtualMachineStatusStopped {
		err := fmt.Errorf("instance still running")
		log.Err(err).Str("namespace", namespace).Str("name", name).Str("status", string(vm.Status.PrintableStatus)).Msg("Cannot unmount disk on running instance")
		return err
	}

	newVolumes := make([]v1.Volume, 0)
	for _, volume := range vm.Spec.Template.Spec.Volumes {
		if volume.Name != diskEID {
			newVolumes = append(newVolumes, volume)
		}
	}
	vm.Spec.Template.Spec.Volumes = newVolumes

	newDisk := make([]v1.Disk, 0)
	for _, disk := range vm.Spec.Template.Spec.Domain.Devices.Disks {
		if disk.Name != diskEID {
			newDisk = append(newDisk, disk)
		}
	}
	vm.Spec.Template.Spec.Domain.Devices.Disks = newDisk

	// Update the VM
	_, err = config.VirtClient.VirtualMachine(vm.Namespace).Update(ctx, vm, k8smetav1.UpdateOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Str("disk", diskEID).Msg("Failed to unmount disk from VM")
		return err
	}

	return nil
}

func checkDiskOrder(vm *v1.VirtualMachine, diskList []Disk) error {
	// Filter to only PVC-backed volumes (actual disks), ignoring cloud-init,
	// container disks, and other non-disk volume types.
	var diskVolumes []v1.Volume
	for _, vol := range vm.Spec.Template.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			diskVolumes = append(diskVolumes, vol)
		}
	}

	if len(diskVolumes) > len(diskList) {
		return fmt.Errorf("some disks have been removed")
	}

	for idx, volume := range diskVolumes {
		if diskList[idx].Eid != volume.Name {
			return fmt.Errorf("disk eid mismatch - order change")
		}
	}

	return nil
}
