package vm

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	v1 "kubevirt.io/api/core/v1"
)

func IsVMInSubnet(ctx context.Context, namespace, subnetName string) (bool, error) {
	log := logger.GetLogger(ctx)
	vms, err := config.VirtClient.VirtualMachine(namespace).List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("error listing vms")
		return false, err
	}

	for _, vm := range vms.Items {
		annotation := fmt.Sprintf("%s.%s.ovn.kubernetes.io/port_security", subnetName, namespace)
		if vm.Spec.Template.ObjectMeta.Annotations[annotation] == "true" {
			return true, nil
		}
	}

	return false, nil
}

// IsVMMountDisk return if a Disk is mounted in a VM
// If not return false
// If it's mounted return true and the EID of the VM where the disk is mounted
func IsVMMountDisk(ctx context.Context, namespace, diskName string) (bool, string, error) {
	log := logger.GetLogger(ctx)

	list, err := informers.WatcherSet[informers.VirtualMachine].ByIndex("namespace", namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("error listing vms")
		return false, "", err
	}

	for _, item := range list {
		var vm v1.VirtualMachine
		if err := utils.UnstructuredToStruct(item.(*unstructured.Unstructured), &vm); err != nil {
			return false, "", err
		}
		for _, disk := range vm.Spec.Template.Spec.Volumes {
			if (disk.PersistentVolumeClaim != nil && disk.PersistentVolumeClaim.ClaimName == diskName) ||
				(disk.DataVolume != nil && disk.DataVolume.Name == diskName) {
				return true, vm.Name, nil
			}
		}
	}

	return false, "", nil
}

// IsVMListMountDisk return if a Disk is mounted in a VM from the list
// If not return false
// If it's mounted return true and the EID of the VM where the disk is mounted
func IsVMListMountDisk(vms []view.VirtualMachineView, diskName string) (bool, string) {
	for _, vm := range vms {
		for _, disk := range vm.Spec.Template.Spec.Volumes {
			if (disk.PersistentVolumeClaim != nil && disk.PersistentVolumeClaim.ClaimName == diskName) ||
				(disk.DataVolume != nil && disk.DataVolume.Name == diskName) {
				return true, vm.Name
			}
		}
	}

	return false, ""
}

func StartVM(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)
	err := config.VirtClient.VirtualMachine(namespace).Start(ctx, name, &v1.StartOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error starting vm")
		return err
	}
	return nil
}

func StopVM(ctx context.Context, namespace, name string, force bool) error {
	log := logger.GetLogger(ctx)
	gracePeriod := int64(180)

	if force {
		gracePeriod = 0
	}

	err := config.VirtClient.VirtualMachine(namespace).Stop(ctx, name, &v1.StopOptions{
		GracePeriod: &gracePeriod,
	})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error stopping vm")
		return err
	}
	return nil
}

func RestartVM(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)
	err := config.VirtClient.VirtualMachine(namespace).Restart(ctx, name, &v1.RestartOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error restarting vm")
		return err
	}
	return nil
}
