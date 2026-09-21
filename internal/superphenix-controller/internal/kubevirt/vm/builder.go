package vm

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s/ssh"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/subnet"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v2 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	v1 "kubevirt.io/api/core/v1"
)

func withDataVolumeDisks(ctx context.Context, vm *v1.VirtualMachine, diskList []Disk) error {
	sort.Slice(diskList, func(i, j int) bool {
		return diskList[i].Order < diskList[j].Order
	})

	for _, disk := range diskList {
		// Create new disk
		if !disk.Disk.IsEmpty() {
			err := disk.Disk.CreateDisk(ctx, vm.Namespace)
			if err != nil {
				return err
			}

			disk.Eid = disk.Disk.GetResourceEffectiveID()
		}

		vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, v1.Volume{
			Name: disk.Eid,
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: v2.PersistentVolumeClaimVolumeSource{
						ClaimName: disk.Eid,
					},
				},
			},
		})

		if disk.Cdrom {
			ro := true

			cdrom := v1.CDRomTarget{ReadOnly: &ro}
			if disk.Bus == "virtio" {
				cdrom = v1.CDRomTarget{ReadOnly: &ro, Bus: v1.DiskBusVirtio}
			} else {
				cdrom = v1.CDRomTarget{ReadOnly: &ro, Bus: v1.DiskBusSATA}

			}

			vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, v1.Disk{
				Name:       disk.Eid,
				DiskDevice: v1.DiskDevice{CDRom: &cdrom},
			})

		} else {

			diskDevice := v1.DiskDevice{}
			if disk.Bus == "sata" {
				diskDevice = v1.DiskDevice{Disk: &v1.DiskTarget{Bus: v1.DiskBusSATA}}
			} else if disk.Bus == "virtio" {
				diskDevice = v1.DiskDevice{Disk: &v1.DiskTarget{Bus: v1.DiskBusVirtio}}
			}

			vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, v1.Disk{
				Name:       disk.Eid,
				DiskDevice: diskDevice,
			})
		}
	}

	return nil
}

func withCloudInit(ctx context.Context, vm *v1.VirtualMachine, init CloudInit) error {
	userData := DEFAULT_CLOUD_INIT
	if init.Custom {
		userData = init.Config
	}

	pwd, err := utils.GeneratePassword(32)
	if err != nil {
		return err
	}

	userData = strings.Replace(userData, "<generated_password>", pwd, 1)

	spxLabels := map[string]string{}
	// Fetch all spx labels
	for k, v := range vm.GetLabels() {
		if strings.HasPrefix(k, spxId.SpxLabelPrefix) {
			spxLabels[k] = v
		}
	}

	secretName, err := CreateOrUpdateCloudInit(ctx, vm.Namespace, vm.Name, userData, spxLabels)
	if err != nil {
		return err
	}

	vm.Spec.Template.Spec.Volumes = append(vm.Spec.Template.Spec.Volumes, v1.Volume{
		Name: "cloud-init",
		VolumeSource: v1.VolumeSource{
			CloudInitNoCloud: &v1.CloudInitNoCloudSource{
				UserDataSecretRef: &v2.LocalObjectReference{Name: secretName},
			},
		},
	})

	diskDevice := v1.DiskDevice{}
	if init.Bus == "sata" {
		diskDevice = v1.DiskDevice{Disk: &v1.DiskTarget{Bus: v1.DiskBusSATA}}
	} else if init.Bus == "virtio" {
		diskDevice = v1.DiskDevice{Disk: &v1.DiskTarget{Bus: v1.DiskBusVirtio}}
	}

	vm.Spec.Template.Spec.Domain.Devices.Disks = append(vm.Spec.Template.Spec.Domain.Devices.Disks, v1.Disk{
		Name:       "cloud-init",
		DiskDevice: diskDevice,
	})

	return nil
}

func withNetworks(ctx context.Context, namespace string, networks []Network, vm *v1.VirtualMachine) error {
	log := logger.GetLogger(ctx)
	// Sort network slice to be sure we set interface in the correct order
	sort.Slice(networks, func(i, j int) bool {
		return networks[i].Order < networks[j].Order
	})

	subnets := subnet.ListSubnet(ctx, namespace)
	subnetsByEId := make(map[string]view.SubnetView, len(subnets))
	for _, s := range subnets {
		subnetsByEId[s.Name] = s
	}

	for index, net := range networks {

		if s, ok := subnetsByEId[net.SubnetEId]; ok {

			// Reject static IPs that are malformed or outside the subnet CIDR
			if err := validateNetworkIP(net.SubnetEId, s.Spec.CIDRBlock, net.IPv4, net.IPv6); err != nil {
				log.Error().Err(err).Any("network", net).Msg("Network not valid, ip out of subnet range")
				return err
			}

			// Reject static MAC that is malformed
			if err := validateNetworkMAC(net.MACAddress); err != nil {
				log.Error().Err(err).Any("network", net).Msg("Network not valid, invalid MAC address")
				return err
			}

			//	Add annotation
			vm.Spec.Template.ObjectMeta.Annotations["kubevirt.io/allow-pod-bridge-network-live-migration"] = "true"
			annotationLiveMig := fmt.Sprintf("%s.%s.ovn.kubernetes.io/allow_live_migration", net.SubnetEId, namespace)
			vm.Spec.Template.ObjectMeta.Annotations[annotationLiveMig] = "true"

			// Static IP
			if net.IPv4 != "" && net.IPv6 != "" {
				ipAnnotation := fmt.Sprintf("%s.%s.ovn.kubernetes.io/ip_address", net.SubnetEId, namespace)
				vm.Spec.Template.ObjectMeta.Annotations[ipAnnotation] = fmt.Sprintf("%s,%s", net.IPv4, net.IPv6)
			} else if net.IPv4 != "" {
				ipAnnotation := fmt.Sprintf("%s.%s.ovn.kubernetes.io/ip_address", net.SubnetEId, namespace)
				vm.Spec.Template.ObjectMeta.Annotations[ipAnnotation] = net.IPv4
			} else if net.IPv6 != "" {
				ipAnnotation := fmt.Sprintf("%s.%s.ovn.kubernetes.io/ip_address", net.SubnetEId, namespace)
				vm.Spec.Template.ObjectMeta.Annotations[ipAnnotation] = net.IPv6
			}

			// Static MAC
			if net.MACAddress != "" {
				macAnnotation := fmt.Sprintf("%s.%s.ovn.kubernetes.io/mac_address", net.SubnetEId, namespace)
				vm.Spec.Template.ObjectMeta.Annotations[macAnnotation] = net.MACAddress
			}

			netInterface := v1.Interface{
				Name:       fmt.Sprintf("interface-%d", net.Order),
				Binding:    &v1.PluginBinding{Name: "managedtap"},
				MacAddress: net.MACAddress,
			}

			if net.Enabled != nil && !*net.Enabled {
				netInterface.State = v1.InterfaceStateLinkDown
			} else {
				netInterface.State = v1.InterfaceStateLinkUp
			}

			if net.Model == "virtio" || net.Model == "e1000" {
				netInterface.Model = net.Model
			}

			//	Add devices interface
			vm.Spec.Template.Spec.Domain.Devices.Interfaces = append(vm.Spec.Template.Spec.Domain.Devices.Interfaces, netInterface)

			//	Add network declaration
			vm.Spec.Template.Spec.Networks = append(vm.Spec.Template.Spec.Networks, v1.Network{
				Name: fmt.Sprintf("interface-%d", net.Order),
				NetworkSource: v1.NetworkSource{
					Multus: &v1.MultusNetwork{
						NetworkName: fmt.Sprintf("%s/%s", namespace, net.SubnetEId),
						Default:     index == 0, // Set the first network as Default Network (we can only have one default network in Multus)
					},
				},
			})

		} else {
			log.Error().Str("subnet not found", net.SubnetEId).Any("network", net).Msg("Network not valid, subnet not found")
			return fmt.Errorf("network %s not valid, subnet not found", net.SubnetEId)
		}
	}
	return nil
}

// withAdvancedOptions applies the optional device/firmware overrides onto the VM
// spec. Nil adv is a no-op (existing values are preserved). Otherwise adv is
// authoritative for the fields it carries: a nil leaf clears the override
// (inherit), a non-nil leaf forces the value.
func withAdvancedOptions(vm *v1.VirtualMachine, adv *AdvancedOptionsInput) {
	if adv == nil {
		return
	}

	devices := &vm.Spec.Template.Spec.Domain.Devices

	// TPM
	tpmEnabled := adv.Devices.Tpm.Enabled
	tpmPersistent := adv.Devices.Tpm.Persistent
	switch {
	case tpmEnabled == nil && tpmPersistent == nil:
		// inherit
		devices.TPM = nil
	case tpmEnabled != nil && !*tpmEnabled:
		// remove the TPM
		devices.TPM = &v1.TPMDevice{Enabled: boolPtr(false)}
	default:
		tpm := &v1.TPMDevice{}
		if tpmEnabled != nil {
			tpm.Enabled = boolPtr(*tpmEnabled)
		}
		if tpmPersistent != nil {
			tpm.Persistent = boolPtr(*tpmPersistent)
		}
		devices.TPM = tpm
	}

	// Firmware / EFI block. enabled drives the bootloader mode; the sub-fields
	// configure it when present. Mirrors the TPM handler. No pinning - leaving
	// the EFI override off lets kubevirt apply PreferredEfi (which it skips
	// wholesale once any EFI or BIOS block is present on the spec).
	efiEnabled := adv.Firmware.Bootloader.Efi.Enabled
	secureBoot := adv.Firmware.Bootloader.Efi.SecureBoot
	persistent := adv.Firmware.Bootloader.Efi.Persistent
	switch {
	case efiEnabled != nil && !*efiEnabled:
		// off: force BIOS, dropping any EFI override and the SMM we add.
		forceBIOS(vm)
	case efiEnabled == nil && secureBoot == nil && persistent == nil:
		// inherit: drop our EFI override so PreferredEfi/PreferredSmm govern.
		clearEFIOverride(vm)
	default:
		// on: EFI bootloader present with concrete (or defaulted) fields.
		ensureEFI(vm)
		efi := vm.Spec.Template.Spec.Domain.Firmware.Bootloader.EFI
		efi.SecureBoot, efi.Persistent = nil, nil
		if secureBoot != nil {
			efi.SecureBoot = boolPtr(*secureBoot)
		}
		if persistent != nil {
			efi.Persistent = boolPtr(*persistent)
		}
		// An EFI block present means Secure Boot is on unless explicitly false
		// (kubevirt defaults it to true), and Secure Boot requires SMM. Mirror
		// virt-launcher's rule so the VM is never rejected.
		if efi.SecureBoot == nil || *efi.SecureBoot {
			ensureSMMEnabled(vm)
		} else {
			clearSMM(vm)
		}
	}

	// SMBIOS
	if smbios := adv.Firmware.SMBIOS; smbios != nil {
		ensureFirmware(vm)
		fw := vm.Spec.Template.Spec.Domain.Firmware
		if smbios.Serial != "" {
			fw.Serial = smbios.Serial
		}
		if smbios.UUID != "" {
			fw.UUID = types.UID(smbios.UUID)
		}
	}
}

// clearEFIOverride drops our bootloader override (EFI on, or BIOS off) and the
// SMM we add for Secure Boot, returning firmware to the preference-governed
// state. Firmware fields we never set (UUID, ...) are preserved.
func clearEFIOverride(vm *v1.VirtualMachine) {
	if fw := vm.Spec.Template.Spec.Domain.Firmware; fw != nil {
		fw.Bootloader = nil
	}
	clearSMM(vm)
}

// clearSMM drops the SMM feature; we only ever enable it for Secure Boot.
func clearSMM(vm *v1.VirtualMachine) {
	if feat := vm.Spec.Template.Spec.Domain.Features; feat != nil {
		feat.SMM = nil
	}
}

// forceBIOS pins the BIOS bootloader, dropping any EFI override and the SMM we
// add for Secure Boot. A present BIOS bootloader makes kubevirt skip the
// instance type's PreferredEfi, so this overrides a preference that wants EFI.
func forceBIOS(vm *v1.VirtualMachine) {
	ensureFirmwareBootloader(vm)
	bl := vm.Spec.Template.Spec.Domain.Firmware.Bootloader
	bl.EFI = nil
	if bl.BIOS == nil {
		bl.BIOS = &v1.BIOS{}
	}
	clearSMM(vm)
}

// ensureFirmware makes sure Firmware exists.
func ensureFirmware(vm *v1.VirtualMachine) {
	if vm.Spec.Template.Spec.Domain.Firmware == nil {
		vm.Spec.Template.Spec.Domain.Firmware = &v1.Firmware{}
	}
}

// ensureFirmwareBootloader makes sure Firmware and Bootloader exist.
func ensureFirmwareBootloader(vm *v1.VirtualMachine) {
	fw := vm.Spec.Template.Spec.Domain.Firmware
	if fw == nil {
		fw = &v1.Firmware{}
		vm.Spec.Template.Spec.Domain.Firmware = fw
	}
	if fw.Bootloader == nil {
		fw.Bootloader = &v1.Bootloader{}
	}
}

func ensureEFI(vm *v1.VirtualMachine) {
	ensureFirmwareBootloader(vm)
	bl := vm.Spec.Template.Spec.Domain.Firmware.Bootloader
	// EFI and BIOS are mutually exclusive; drop any BIOS a prior "Off" forced,
	// else kubevirt's validating webhook rejects a bootloader with both.
	bl.BIOS = nil
	if bl.EFI == nil {
		bl.EFI = &v1.EFI{}
	}
}

func ensureSMMEnabled(vm *v1.VirtualMachine) {
	features := vm.Spec.Template.Spec.Domain.Features
	if features == nil {
		features = &v1.Features{}
		vm.Spec.Template.Spec.Domain.Features = features
	}
	if features.SMM == nil {
		features.SMM = &v1.FeatureState{}
	}
	features.SMM.Enabled = boolPtr(true)
}

func withSSHKeys(ctx context.Context, namespace string, sshKeys []string, vm *v1.VirtualMachine) error {
	log := logger.GetLogger(ctx)
	sshList, err := ssh.ListSSHKey(ctx, namespace)
	if err != nil {
		log.Err(err).Msg("SSH Keys cannot be listed")
		return fmt.Errorf("ssh keys cannot be listed")
	}
	sshEIds := make([]string, 0)
	for _, s := range sshList {
		sshEIds = append(sshEIds, s.Name)
	}

	vm.Spec.Template.Spec.AccessCredentials = make([]v1.AccessCredential, 0)
	for _, key := range sshKeys {
		if slices.Contains(sshEIds, key) {
			vm.Spec.Template.Spec.AccessCredentials = append(vm.Spec.Template.Spec.AccessCredentials, v1.AccessCredential{
				SSHPublicKey: &v1.SSHPublicKeyAccessCredential{
					PropagationMethod: v1.SSHPublicKeyAccessCredentialPropagationMethod{
						NoCloud: &v1.NoCloudSSHPublicKeyAccessCredentialPropagation{},
					},
					Source: v1.SSHPublicKeyAccessCredentialSource{
						Secret: &v1.AccessCredentialSecretSource{
							SecretName: key,
						},
					},
				},
			})
		} else {
			log.Error().Str("ssh key not found", key).Msg("SSH Key not found for the user")
			return fmt.Errorf("ssh key %s not found for the user", key)
		}
	}
	return nil
}
