package view

import (
	instancetypev1beta1 "kubevirt.io/api/instancetype/v1beta1"
)

// VirtualMachinePreferenceView exposes relevant fields from a VirtualMachineClusterPreference spec.
type VirtualMachinePreferenceView struct {
	// Name is the name of the VirtualMachineClusterPreference
	Name string `json:"name"`
	// Clock optionally defines preferences associated with the Clock attribute
	// +optional
	Clock *instancetypev1beta1.ClockPreferences `json:"clock,omitempty"`
	// CPU optionally defines preferences associated with the CPU attribute
	// +optional
	CPU *instancetypev1beta1.CPUPreferences `json:"cpu,omitempty"`
	// Devices optionally defines preferences associated with the Devices attribute
	// +optional
	Devices *instancetypev1beta1.DevicePreferences `json:"devices,omitempty"`
	// Features optionally defines preferences associated with the Features attribute
	// +optional
	Features *instancetypev1beta1.FeaturePreferences `json:"features,omitempty"`
	// Firmware optionally defines preferences associated with the Firmware attribute
	// +optional
	Firmware *instancetypev1beta1.FirmwarePreferences `json:"firmware,omitempty"`
	// Machine optionally defines preferences associated with the Machine attribute
	// +optional
	Machine *instancetypev1beta1.MachinePreferences `json:"machine,omitempty"`
	// Volumes optionally defines preferences associated with the Volumes attribute
	// +optional
	Volumes *instancetypev1beta1.VolumePreferences `json:"volumes,omitempty"`
}

// VMClusterPreferenceToView converts a VirtualMachineClusterPreference to a VirtualMachinePreferenceView.
func VMClusterPreferenceToView(name string, spec instancetypev1beta1.VirtualMachinePreferenceSpec) VirtualMachinePreferenceView {
	return VirtualMachinePreferenceView{
		Name:     name,
		Clock:    spec.Clock,
		CPU:      spec.CPU,
		Devices:  spec.Devices,
		Features: spec.Features,
		Firmware: spec.Firmware,
		Machine:  spec.Machine,
		Volumes:  spec.Volumes,
	}
}
