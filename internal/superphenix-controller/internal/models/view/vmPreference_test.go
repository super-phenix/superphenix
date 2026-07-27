package view

import (
	"reflect"
	"testing"

	v1 "kubevirt.io/api/core/v1"
	instancetypev1beta1 "kubevirt.io/api/instancetype/v1beta1"
)

func TestVMClusterPreferenceToView(t *testing.T) {
	preferredCPUTopology := instancetypev1beta1.Sockets
	preferredMachineType := "q35"
	preferredAutoattachGraphicsDevice := true

	tests := []struct {
		name     string
		prefName string
		spec     instancetypev1beta1.VirtualMachinePreferenceSpec
		expected VirtualMachinePreferenceView
	}{
		{
			name:     "empty spec",
			prefName: "empty-pref",
			spec:     instancetypev1beta1.VirtualMachinePreferenceSpec{},
			expected: VirtualMachinePreferenceView{
				Name: "empty-pref",
			},
		},
		{
			name:     "spec with all fields",
			prefName: "full-pref",
			spec: instancetypev1beta1.VirtualMachinePreferenceSpec{
				Clock: &instancetypev1beta1.ClockPreferences{
					PreferredClockOffset: &v1.ClockOffset{},
				},
				CPU: &instancetypev1beta1.CPUPreferences{
					PreferredCPUTopology: &preferredCPUTopology,
				},
				Devices: &instancetypev1beta1.DevicePreferences{
					PreferredAutoattachGraphicsDevice: &preferredAutoattachGraphicsDevice,
				},
				Features: &instancetypev1beta1.FeaturePreferences{},
				Firmware: &instancetypev1beta1.FirmwarePreferences{},
				Machine: &instancetypev1beta1.MachinePreferences{
					PreferredMachineType: preferredMachineType,
				},
				Volumes: &instancetypev1beta1.VolumePreferences{},
			},
			expected: VirtualMachinePreferenceView{
				Name: "full-pref",
				Clock: &instancetypev1beta1.ClockPreferences{
					PreferredClockOffset: &v1.ClockOffset{},
				},
				CPU: &instancetypev1beta1.CPUPreferences{
					PreferredCPUTopology: &preferredCPUTopology,
				},
				Devices: &instancetypev1beta1.DevicePreferences{
					PreferredAutoattachGraphicsDevice: &preferredAutoattachGraphicsDevice,
				},
				Features: &instancetypev1beta1.FeaturePreferences{},
				Firmware: &instancetypev1beta1.FirmwarePreferences{},
				Machine: &instancetypev1beta1.MachinePreferences{
					PreferredMachineType: preferredMachineType,
				},
				Volumes: &instancetypev1beta1.VolumePreferences{},
			},
		},
		{
			name:     "spec with partial fields",
			prefName: "partial-pref",
			spec: instancetypev1beta1.VirtualMachinePreferenceSpec{
				CPU: &instancetypev1beta1.CPUPreferences{
					PreferredCPUTopology: &preferredCPUTopology,
				},
				Machine: &instancetypev1beta1.MachinePreferences{
					PreferredMachineType: preferredMachineType,
				},
			},
			expected: VirtualMachinePreferenceView{
				Name: "partial-pref",
				CPU: &instancetypev1beta1.CPUPreferences{
					PreferredCPUTopology: &preferredCPUTopology,
				},
				Machine: &instancetypev1beta1.MachinePreferences{
					PreferredMachineType: preferredMachineType,
				},
			},
		},
		{
			name:     "non-mapped fields are excluded",
			prefName: "filtered-pref",
			spec: instancetypev1beta1.VirtualMachinePreferenceSpec{
				Annotations: map[string]string{"key": "value"},
				CPU: &instancetypev1beta1.CPUPreferences{
					PreferredCPUTopology: &preferredCPUTopology,
				},
			},
			expected: VirtualMachinePreferenceView{
				Name: "filtered-pref",
				CPU: &instancetypev1beta1.CPUPreferences{
					PreferredCPUTopology: &preferredCPUTopology,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VMClusterPreferenceToView(tt.prefName, tt.spec)
			if !reflect.DeepEqual(tt.expected, result) {
				t.Errorf("expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}
