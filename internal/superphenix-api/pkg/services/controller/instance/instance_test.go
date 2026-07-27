package instance

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
