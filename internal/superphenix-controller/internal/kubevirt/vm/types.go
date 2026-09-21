package vm

import (
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubevirt/datavolume"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

type Network struct {
	Order      int    `json:"order"`
	SubnetEId  string `json:"subnetEId"` // Subnet Effective ID
	Model      string `json:"model"`
	Enabled    *bool  `json:"enabled,omitempty"`
	IPv4       string `json:"ipv4,omitempty"`
	IPv6       string `json:"ipv6,omitempty"`
	MACAddress string `json:"macAddress,omitempty"`
}

type Disk struct {
	Order int                       `json:"order"`
	Cdrom bool                      `json:"cdrom"`
	Bus   string                    `json:"bus"`
	Eid   string                    `json:"eid,omitempty"`
	Disk  datavolume.CreateDiskInfo `json:"disk,omitempty"`
}

type CloudInit struct {
	Custom bool   `json:"custom"`
	Bus    string `json:"bus"`
	Config string `json:"config,omitempty"`
}

// BlockToggle is the enable baseline every advanced-options block embeds: a
// tri-state toggle. The anonymous embed promotes `enabled` into the block's own
// JSON object, so the wire shape stays {enabled, ...block fields}.
//   - nil   = inherit (use the preference / kubevirt default)
//   - true  = on / force present
//   - false = off (TPM: remove the device; EFI: force BIOS)
type BlockToggle struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// TPMOptions overrides the vTPM device. enabled: nil=inherit, true=add, false=remove.
type TPMOptions struct {
	BlockToggle
	Persistent *bool `json:"persistent,omitempty"`
}

// EFIOptions overrides the EFI firmware. enabled: nil=inherit, true=EFI, false=BIOS.
type EFIOptions struct {
	BlockToggle
	SecureBoot *bool `json:"secureBoot,omitempty"`
	Persistent *bool `json:"persistent,omitempty"` // persist EFI NVRAM across reboots
}

type AdvancedDevices struct {
	Tpm TPMOptions `json:"tpm"`
}

type AdvancedBootloader struct {
	Efi EFIOptions `json:"efi"`
}

type AdvancedFirmware struct {
	Bootloader AdvancedBootloader `json:"bootloader"`
	SMBIOS     *AdvancedSMBIOS    `json:"smbios,omitempty"`
}

type AdvancedSMBIOS struct {
	Serial string `json:"serial,omitempty"`
	UUID   string `json:"uuid,omitempty"`
}

// AdvancedOptionsInput carries optional device/firmware overrides. Its shape
// mirrors the kubevirt spec tree (spec.template.spec.domain.{devices,firmware}).
// A nil *AdvancedOptionsInput means no override at all (see withAdvancedOptions).
type AdvancedOptionsInput struct {
	Devices  AdvancedDevices  `json:"devices"`
	Firmware AdvancedFirmware `json:"firmware"`
}

type CreateVMInfo struct {
	spxId.Metadata

	General struct {
		RunStrategy string   `json:"runStrategy"`
		VMType      string   `json:"vmType"`
		Labels      []string `json:"labels"`
	} `json:"general"`
	Compute struct {
		Cpu    int `json:"cpu"`
		Memory int `json:"memory"`
	} `json:"compute"`
	Network   []Network `json:"network,omitempty"`
	Disks     []Disk    `json:"disks,omitempty"`
	CloudInit CloudInit `json:"cloudInit"`
	SSHKeys   []string  `json:"sshKeys,omitempty"`
	// ContainerDisks holds catalog IDs. Nil falls back to the recommended
	// catalog set for the VM preference; a non-nil slice (including empty) is
	// honoured verbatim. IDs are resolved against this AZ's config catalog.
	ContainerDisks *[]string `json:"containerDisks,omitempty"`
	// Advanced is optional: nil pushes no device/firmware override.
	Advanced *AdvancedOptionsInput `json:"advanced,omitempty"`
}

type UpdateVMInfo struct {
	General struct {
		RunStrategy string   `json:"runStrategy"`
		VMType      string   `json:"vmType"`
		Labels      []string `json:"labels"`
	} `json:"general"`
	Compute struct {
		Cpu    int `json:"cpu"`
		Memory int `json:"memory"`
	} `json:"compute"`
	Network   []Network `json:"network,omitempty"`
	Disks     []Disk    `json:"disks,omitempty"`
	CloudInit CloudInit `json:"cloudInit"`
	SSHKeys   []string  `json:"sshKeys,omitempty"`
	// ContainerDisks is the desired mount state as catalog IDs. Nil preserves
	// whatever is currently attached; non-nil sets it explicitly (empty
	// detaches all). IDs are resolved against this AZ's config catalog.
	ContainerDisks *[]string `json:"containerDisks,omitempty"`
	// Advanced is optional: nil leaves existing device/firmware overrides untouched.
	Advanced *AdvancedOptionsInput `json:"advanced,omitempty"`
}
