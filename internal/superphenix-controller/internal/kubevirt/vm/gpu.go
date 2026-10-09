package vm

import (
	"fmt"
	"sort"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	v1 "kubevirt.io/api/core/v1"
)

// errInvalidGpuPrefix maps GPU resolution failures to a 400 in the HTTP handlers.
const errInvalidGpuPrefix = "invalid gpu"

// maxGpusPerVM is the number of passthrough GPUs a VM may request.
const maxGpusPerVM = 1

// GPU is one requested passthrough GPU, identified by its class (a key of the
// AZ's deviceMapping).
type GPU struct {
	Device string `json:"device"`
}

// GpuClassList returns this AZ's GPU classes sorted by ID, normalising nil to
// an empty slice.
func GpuClassList() []config.GpuClassEntry {
	mapping := config.Global.ProductsConfig.Compute.DeviceMapping
	result := make([]config.GpuClassEntry, 0, len(mapping))
	for _, entry := range mapping {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// resolveGPUs converts the requested GPU classes into KubeVirt GPU devices,
// named gpu-{index} like the sfs-iaas chart.
func resolveGPUs(gpus []GPU) ([]v1.GPU, error) {
	if len(gpus) > maxGpusPerVM {
		return nil, fmt.Errorf("%s: %d gpus requested, at most %d allowed", errInvalidGpuPrefix, len(gpus), maxGpusPerVM)
	}
	mapping := config.Global.ProductsConfig.Compute.DeviceMapping
	out := make([]v1.GPU, 0, len(gpus))
	for i, gpu := range gpus {
		if gpu.Device == "" {
			return nil, fmt.Errorf("%s: empty class at index %d", errInvalidGpuPrefix, i)
		}
		entry, ok := mapping[gpu.Device]
		if !ok {
			return nil, fmt.Errorf("%s: unknown class %q", errInvalidGpuPrefix, gpu.Device)
		}
		out = append(out, v1.GPU{
			Name:       fmt.Sprintf("gpu-%d", i),
			DeviceName: entry.DeviceName,
		})
	}
	return out, nil
}

// setGPUs sets the VM's passthrough GPUs from a list returned by resolveGPUs.
// The list is authoritative: an empty list removes every GPU.
//
// GPU VMs use the host-passthrough CPU model: it exposes the host's physical
// address width, so OVMF sizes its 64-bit MMIO window for large GPU BARs (the
// RTX PRO 6000 needs 128 GB). With the cluster CPU model the window stays at
// ~32 GB and UEFI guests hang at boot or cannot use the GPU. GPU VMs cannot
// live-migrate anyway, so host-passthrough costs nothing here.
func setGPUs(vm *v1.VirtualMachine, gpus []v1.GPU) {
	domain := &vm.Spec.Template.Spec.Domain
	if len(gpus) == 0 {
		domain.Devices.GPUs = nil
		// Only undo what we set: another CPU model is left untouched.
		if domain.CPU != nil && domain.CPU.Model == v1.CPUModeHostPassthrough {
			domain.CPU.Model = ""
		}
		return
	}
	domain.Devices.GPUs = gpus
	if domain.CPU == nil {
		domain.CPU = &v1.CPU{}
	}
	domain.CPU.Model = v1.CPUModeHostPassthrough
}
