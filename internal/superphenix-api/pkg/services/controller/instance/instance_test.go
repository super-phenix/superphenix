package instance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/super-phenix/superphenix/pkg/utils/validation"
)

func strSlicePtr(s []string) *[]string { return &s }

// TestToUpdateAzControllerBody_ForwardsContainerDisks covers the pointer
// pass-through semantics (nil = preserve, empty = unmount all). The API
// forwards the catalog IDs verbatim; the AZ controller does the resolution.
func TestToUpdateAzControllerBody_ForwardsContainerDisks(t *testing.T) {
	tests := []struct {
		name string
		in   *[]string
	}{
		{name: "nil preserves current state (passes through)", in: nil},
		{name: "explicit empty unmounts everything", in: strSlicePtr([]string{})},
		{name: "explicit ids forwarded", in: strSlicePtr([]string{"windows-virtio-drivers"})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := UpdateInstanceBody{ContainerDisks: tt.in}

			got := toUpdateAzControllerBody(body, nil)

			assert.Equal(t, tt.in, got.ContainerDisks)
		})
	}
}

func TestToUpdateAzControllerBody_RoundTripsAllFields(t *testing.T) {
	body := UpdateInstanceBody{
		Network: []InstanceNetworkBody{
			{Order: 0, SubnetEId: "subnet-eid", Model: "virtio", IPv4: "10.0.0.5"},
		},
		CloudInit:      InstanceCloudInitBody{Custom: true, Bus: "virtio", Config: "#cloud-config"},
		SSHKeys:        []string{"ssh-rsa AAA"},
		ContainerDisks: &[]string{"windows-virtio-drivers"},
	}
	body.General.ProductName = "vm-1"
	body.General.RunStrategy = "Always"
	body.General.VMType = "windows-server-2022"
	body.General.Labels = []string{"env=prod"}
	body.Compute.Cpu = 4
	body.Compute.Memory = 8192

	disksForCtrl := []InstanceDiskSpxControllerBody{
		{Order: 0, Cdrom: false, Bus: "virtio", Eid: "disk-eid"},
	}

	got := toUpdateAzControllerBody(body, disksForCtrl)

	assert.Equal(t, body.General.RunStrategy, got.General.RunStrategy)
	assert.Equal(t, body.General.VMType, got.General.VMType)
	assert.Equal(t, body.General.Labels, got.General.Labels)
	assert.Equal(t, body.Compute, got.Compute)
	assert.Equal(t, body.Network, got.Network)
	assert.Equal(t, disksForCtrl, got.Disks)
	assert.Equal(t, body.CloudInit, got.CloudInit)
	assert.Equal(t, body.SSHKeys, got.SSHKeys)
	assert.Equal(t, body.ContainerDisks, got.ContainerDisks)
}

// TestResolveMountedContainerDisks walks a raw controller response and confirms
// every ContainerDisk-sourced volume name is returned (the volume name is the
// catalog ID; the API no longer holds a catalog to intersect against).
func TestResolveMountedContainerDisks(t *testing.T) {
	buildVM := func(volumes []map[string]any) map[string]any {
		volsAny := make([]any, 0, len(volumes))
		for _, v := range volumes {
			volsAny = append(volsAny, v)
		}
		return map[string]any{
			"vm": map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"volumes": volsAny,
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name    string
		volumes []map[string]any
		want    []string
	}{
		{
			name:    "no volumes returns empty",
			volumes: nil,
			want:    []string{},
		},
		{
			name: "container disk volume returned",
			volumes: []map[string]any{
				{"name": "windows-virtio-drivers", "containerDisk": map[string]any{"image": "img"}},
			},
			want: []string{"windows-virtio-drivers"},
		},
		{
			name: "every container disk volume returned (no catalog filter)",
			volumes: []map[string]any{
				{"name": "ad-hoc-iso", "containerDisk": map[string]any{"image": "img"}},
			},
			want: []string{"ad-hoc-iso"},
		},
		{
			name: "DataVolume source skipped",
			volumes: []map[string]any{
				{"name": "windows-virtio-drivers", "dataVolume": map[string]any{"name": "dv-x"}},
			},
			want: []string{},
		},
		{
			name: "mix of relevant and unrelated volumes",
			volumes: []map[string]any{
				{"name": "rootfs", "dataVolume": map[string]any{"name": "rootfs-dv"}},
				{"name": "windows-virtio-drivers", "containerDisk": map[string]any{"image": "img"}},
				{"name": "ad-hoc-iso", "containerDisk": map[string]any{"image": "img2"}},
			},
			want: []string{"windows-virtio-drivers", "ad-hoc-iso"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveMountedContainerDisks(buildVM(tt.volumes))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInstanceNetworkBody_EnabledSerialization(t *testing.T) {
	tests := []struct {
		name          string
		jsonInput     string
		expectedState *bool
		expectInWire  string
		rejectInWire  string
	}{
		{
			name:          "omitted enabled defaults to nil",
			jsonInput:     `{"order":0,"subnetEId":"sub-1","model":"virtio"}`,
			expectedState: nil,
			rejectInWire:  `"enabled"`,
		},
		{
			name:          "explicitly enabled true",
			jsonInput:     `{"order":0,"subnetEId":"sub-1","model":"virtio","enabled":true}`,
			expectedState: boolPtr(true),
			expectInWire:  `"enabled":true`,
		},
		{
			name:          "explicitly enabled false",
			jsonInput:     `{"order":0,"subnetEId":"sub-1","model":"virtio","enabled":false}`,
			expectedState: boolPtr(false),
			expectInWire:  `"enabled":false`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var net InstanceNetworkBody
			err := json.Unmarshal([]byte(tt.jsonInput), &net)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedState, net.Enabled)

			marshaled, err := json.Marshal(net)
			require.NoError(t, err)

			if tt.expectInWire != "" {
				assert.Contains(t, string(marshaled), tt.expectInWire)
			}
			if tt.rejectInWire != "" {
				assert.NotContains(t, string(marshaled), tt.rejectInWire)
			}
		})
	}
}

func TestToUpdateAzControllerBody_ForwardsNetworkEnabled(t *testing.T) {
	tests := []struct {
		name       string
		enabled    *bool
		expectWire string
		rejectWire string
	}{
		{
			name:       "nil enabled forwarded as nil",
			enabled:    nil,
			rejectWire: `"enabled"`,
		},
		{
			name:       "true enabled forwarded as true",
			enabled:    boolPtr(true),
			expectWire: `"enabled":true`,
		},
		{
			name:       "false enabled forwarded as false",
			enabled:    boolPtr(false),
			expectWire: `"enabled":false`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := UpdateInstanceBody{
				Network: []InstanceNetworkBody{
					{Order: 0, SubnetEId: "sub-1", Model: "virtio", Enabled: tt.enabled},
				},
			}

			got := toUpdateAzControllerBody(body, nil)
			require.Len(t, got.Network, 1)
			assert.Equal(t, tt.enabled, got.Network[0].Enabled)

			marshaled, err := json.Marshal(got)
			require.NoError(t, err)

			if tt.expectWire != "" {
				assert.Contains(t, string(marshaled), tt.expectWire)
			}
			if tt.rejectWire != "" {
				assert.NotContains(t, string(marshaled), tt.rejectWire)
			}
		})
	}
}

func TestCreateInstanceSpxControllerBody_NetworkEnabledSerialization(t *testing.T) {
	tests := []struct {
		name       string
		enabled    *bool
		expectWire string
		rejectWire string
	}{
		{
			name:       "nil enabled omitted in create controller payload",
			enabled:    nil,
			rejectWire: `"enabled"`,
		},
		{
			name:       "true enabled preserved in create controller payload",
			enabled:    boolPtr(true),
			expectWire: `"enabled":true`,
		},
		{
			name:       "false enabled preserved in create controller payload",
			enabled:    boolPtr(false),
			expectWire: `"enabled":false`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrlBody := CreateInstanceSpxControllerBody{
				Network: []InstanceNetworkBody{
					{Order: 0, SubnetEId: "sub-1", Model: "virtio", Enabled: tt.enabled},
				},
			}

			marshaled, err := json.Marshal(ctrlBody)
			require.NoError(t, err)

			if tt.expectWire != "" {
				assert.Contains(t, string(marshaled), tt.expectWire)
			}
			if tt.rejectWire != "" {
				assert.NotContains(t, string(marshaled), tt.rejectWire)
			}
		})
	}
}

func TestInstanceBody_NetworkEnabledLifecycle(t *testing.T) {
	tests := []struct {
		name              string
		jsonInput         string
		expectedOrder     []int
		expectedEnabled   []*bool
		expectWireSubstrs []string
		rejectWireSubstrs []string
	}{
		{
			name: "create instance body with omitted, true, and false interfaces",
			jsonInput: `{
				"general": {"productName": "vm-test", "runStrategy": "Always", "vmType": "linux"},
				"compute": {"cpu": 2, "memory": 4096},
				"network": [
					{"order": 0, "subnetEId": "sub-1", "model": "virtio"},
					{"order": 1, "subnetEId": "sub-2", "model": "virtio", "enabled": true},
					{"order": 2, "subnetEId": "sub-3", "model": "virtio", "enabled": false}
				]
			}`,
			expectedOrder:   []int{0, 1, 2},
			expectedEnabled: []*bool{nil, boolPtr(true), boolPtr(false)},
			expectWireSubstrs: []string{
				`"enabled":true`,
				`"enabled":false`,
			},
			rejectWireSubstrs: []string{},
		},
		{
			name: "update instance body with multiple toggled interfaces forwarded to az controller",
			jsonInput: `{
				"network": [
					{"order": 0, "subnetEId": "sub-1", "model": "virtio", "enabled": false},
					{"order": 1, "subnetEId": "sub-2", "model": "virtio"}
				]
			}`,
			expectedOrder:   []int{0, 1},
			expectedEnabled: []*bool{boolPtr(false), nil},
			expectWireSubstrs: []string{
				`"enabled":false`,
			},
			rejectWireSubstrs: []string{
				`"enabled":true`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var updateBody UpdateInstanceBody
			err := json.Unmarshal([]byte(tt.jsonInput), &updateBody)
			require.NoError(t, err)

			require.Len(t, updateBody.Network, len(tt.expectedOrder))
			for i, expOrder := range tt.expectedOrder {
				assert.Equal(t, expOrder, updateBody.Network[i].Order)
				assert.Equal(t, tt.expectedEnabled[i], updateBody.Network[i].Enabled)
			}

			// Verify forwarding to AZ controller preserves enabled states
			ctrlBody := toUpdateAzControllerBody(updateBody, nil)
			require.Len(t, ctrlBody.Network, len(tt.expectedOrder))
			for i, expEnabled := range tt.expectedEnabled {
				assert.Equal(t, expEnabled, ctrlBody.Network[i].Enabled)
			}

			// Verify JSON wire representation
			wireBytes, err := json.Marshal(ctrlBody)
			require.NoError(t, err)
			wireStr := string(wireBytes)

			for _, sub := range tt.expectWireSubstrs {
				assert.Contains(t, wireStr, sub)
			}
			for _, sub := range tt.rejectWireSubstrs {
				assert.NotContains(t, wireStr, sub)
			}

			// Also test CreateInstanceBody unmarshaling if JSON contains general/compute
			var createBody CreateInstanceBody
			if err := json.Unmarshal([]byte(tt.jsonInput), &createBody); err == nil && len(createBody.Network) > 0 {
				require.Len(t, createBody.Network, len(tt.expectedOrder))
				for i, expEnabled := range tt.expectedEnabled {
					assert.Equal(t, expEnabled, createBody.Network[i].Enabled)
				}
				createCtrlBody := CreateInstanceSpxControllerBody{
					Network: createBody.Network,
				}
				createWireBytes, err := json.Marshal(createCtrlBody)
				require.NoError(t, err)
				createWireStr := string(createWireBytes)
				for _, sub := range tt.expectWireSubstrs {
					assert.Contains(t, createWireStr, sub)
				}
				for _, sub := range tt.rejectWireSubstrs {
					assert.NotContains(t, createWireStr, sub)
				}
			}
		})
	}
}

func TestInstanceNetworkBody_MACAddressValidation(t *testing.T) {
	validate := validation.GetValidatorV1().Validator()

	tests := []struct {
		name        string
		macAddress  string
		expectError bool
	}{
		{
			name:        "empty MAC address is valid (omitempty)",
			macAddress:  "",
			expectError: false,
		},
		{
			name:        "valid colon-delimited MAC address",
			macAddress:  "52:54:00:11:22:33",
			expectError: false,
		},
		{
			name:        "reject hyphen-delimited MAC address",
			macAddress:  "52-54-00-11-22-33",
			expectError: true,
		},
		{
			name:        "invalid MAC with non-hex characters",
			macAddress:  "52:54:00:11:22:zz",
			expectError: true,
		},
		{
			name:        "invalid short MAC address",
			macAddress:  "52:54:00:11:22",
			expectError: true,
		},
		{
			name:        "invalid random text",
			macAddress:  "not-a-mac",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			net := InstanceNetworkBody{
				Order:      0,
				SubnetEId:  "sub-1",
				Model:      "virtio",
				MACAddress: tt.macAddress,
			}

			err := validate.Struct(net)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInstanceNetworkBody_MACAddressSerialization(t *testing.T) {
	tests := []struct {
		name         string
		jsonInput    string
		expectedMAC  string
		expectInWire string
		rejectInWire string
	}{
		{
			name:         "omitted MAC is omitted from JSON",
			jsonInput:    `{"order":0,"subnetEId":"sub-1","model":"virtio"}`,
			expectedMAC:  "",
			rejectInWire: `"macAddress"`,
		},
		{
			name:         "provided MAC is preserved in JSON",
			jsonInput:    `{"order":0,"subnetEId":"sub-1","model":"virtio","macAddress":"52:54:00:11:22:33"}`,
			expectedMAC:  "52:54:00:11:22:33",
			expectInWire: `"macAddress":"52:54:00:11:22:33"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var net InstanceNetworkBody
			err := json.Unmarshal([]byte(tt.jsonInput), &net)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedMAC, net.MACAddress)

			marshaled, err := json.Marshal(net)
			require.NoError(t, err)

			if tt.expectInWire != "" {
				assert.Contains(t, string(marshaled), tt.expectInWire)
			}
			if tt.rejectInWire != "" {
				assert.NotContains(t, string(marshaled), tt.rejectInWire)
			}
		})
	}
}
