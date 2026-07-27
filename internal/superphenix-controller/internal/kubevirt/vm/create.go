package vm

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"k8s.io/apimachinery/pkg/api/resource"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/api/core/v1"
)

func CreateVM(ctx context.Context, namespace string, vmInfo CreateVMInfo) error {
	log := logger.GetLogger(ctx)

	metadata := spxId.Metadata{}
	err := metadata.ConvertToSpxMetadata(vmInfo.Metadata)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Any("vmInfo", vmInfo).Msg("Failed to parse vm informations")
		return err
	}

	// Convert to info to Kubevirt Virtual Machine object
	toCreateVm, err := convertToVM(ctx, namespace, vmInfo, metadata)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Any("vmInfo", vmInfo).Msg("Error CreateVMStruct template")
		return err
	}

	// Create the VM
	vmCreated, err := config.VirtClient.VirtualMachine(toCreateVm.Namespace).Create(ctx, toCreateVm, k8smetav1.CreateOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Any("vmInfo", vmInfo).Msg("Error CreateVMStruct creation")
		return err
	}

	if *vmCreated.Spec.RunStrategy == v1.RunStrategyManual {
		// Start the VM
		if err = StartVM(ctx, vmCreated.Namespace, vmCreated.Name); err != nil {
			log.Warn().Err(err).Str("namespace", namespace).Any("vmInfo", vmInfo).Msg("Failed to start VM")
			return nil
		}
	}

	return nil

}

func convertToVM(ctx context.Context, namespace string, vmInfo CreateVMInfo, metadata spxId.Metadata) (*v1.VirtualMachine, error) {
	log := logger.GetLogger(ctx)
	//Validation part
	if !slices.Contains(MEMORY_VALUE_LIST, vmInfo.Compute.Memory) {
		return &v1.VirtualMachine{}, fmt.Errorf("no such memory: %d", vmInfo.Compute.Memory)
	}
	if !slices.Contains(CPU_VALUE_LIST, vmInfo.Compute.Cpu) {
		return &v1.VirtualMachine{}, fmt.Errorf("no such cpu value: %d", vmInfo.Compute.Cpu)
	}

	if !slices.Contains(RUN_STRATEGY_VALUE_LIST, vmInfo.General.RunStrategy) {
		return &v1.VirtualMachine{}, fmt.Errorf("unknown run strategy: %s", vmInfo.General.RunStrategy)
	}

	// Create correct values format
	runStrategy := v1.VirtualMachineRunStrategy(vmInfo.General.RunStrategy)
	memory, err := resource.ParseQuantity(fmt.Sprintf("%dGi", vmInfo.Compute.Memory))
	if err != nil {
		return nil, fmt.Errorf("%s %w", "memory", err)

	}
	cores := uint32(vmInfo.Compute.Cpu)

	terminationGracePeriod := int64(180)

	labels := metadata.GetLabels()
	labels["superphenix.net/workloadClass"] = "virtualmachine"

	customLabels, err := utils.ParseLabels(vmInfo.General.Labels, utils.CustomLabelPrefix)
	if err != nil {
		return nil, fmt.Errorf("error parsing custom labels: %w", err)
	}
	if len(customLabels) > utils.MaxCustomLabelNumber {
		return nil, fmt.Errorf("too many custom labels %d > %d", len(customLabels), utils.MaxCustomLabelNumber)
	}

	// Add custom labels
	maps.Copy(labels, customLabels)

	vmPreference := "linux"
	if vmInfo.General.VMType != "" {
		vmPreference = vmInfo.General.VMType

	}

	// Base Virtual Machine
	vm := &v1.VirtualMachine{
		TypeMeta: k8smetav1.TypeMeta{
			Kind:       v1.VirtualMachineGroupVersionKind.Kind,
			APIVersion: v1.VirtualMachineGroupVersionKind.GroupVersion().String(),
		},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:        metadata.GetResourceEffectiveID(),
			Labels:      labels,
			Annotations: make(map[string]string),
		},
		Spec: v1.VirtualMachineSpec{
			Preference: &v1.PreferenceMatcher{
				Name: vmPreference,
			},
			RunStrategy: &runStrategy,
			Template: &v1.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: k8smetav1.ObjectMeta{
					Labels:      labels,
					Annotations: make(map[string]string),
				},
				Spec: v1.VirtualMachineInstanceSpec{
					Domain: v1.DomainSpec{
						CPU: &v1.CPU{
							Sockets: 1,
							Cores:   cores,
						},
						Memory: &v1.Memory{
							Guest: &memory,
						},
						Devices: v1.Devices{
							Inputs: []v1.Input{
								{
									Bus:  "virtio",
									Type: "tablet",
									Name: "mouse-input",
								},
							},
						},
					},
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Volumes:                       nil,
				},
			},
		},
	}

	// Add namespace
	if namespace != "" {
		vm.Namespace = namespace
	}

	if err := withDataVolumeDisks(ctx, vm, vmInfo.Disks); err != nil {
		log.Err(err).Any("disks", vmInfo.Disks).Msg("Failed to add disks")
		return nil, err
	}

	if err := withCloudInit(ctx, vm, vmInfo.CloudInit); err != nil {
		log.Err(err).Msg("Failed to add cloud init")
		return nil, err
	}

	// Resolve container disk IDs against this AZ's catalog. Nil defaults to the
	// recommended set for the VM preference; non-nil is honoured verbatim.
	var containerDiskSpecs []ContainerDiskSpec
	if vmInfo.ContainerDisks == nil {
		containerDiskSpecs = RecommendedFor(vmInfo.General.VMType)
	} else {
		specs, err := Resolve(*vmInfo.ContainerDisks, vmInfo.General.VMType)
		if err != nil {
			log.Err(err).Msg("Failed to resolve container disks")
			return nil, err
		}
		containerDiskSpecs = specs
	}
	if _, err := applyMounts(vm, containerDiskSpecs); err != nil {
		log.Err(err).Msg("Failed to apply container disks")
		return nil, err
	}

	if err := withNetworks(ctx, namespace, vmInfo.Network, vm); err != nil {
		log.Err(err).Str("namespace", namespace).Any("vmInfo", vmInfo).Msg("Failed to add networks")
		return nil, err
	}

	if err := withSSHKeys(ctx, namespace, vmInfo.SSHKeys, vm); err != nil {
		log.Err(err).Str("namespace", namespace).Any("vmInfo", vmInfo).Msg("Failed to add ssh key")
		return nil, err
	}

	withAdvancedOptions(vm, vmInfo.Advanced)

	return vm, nil
}
