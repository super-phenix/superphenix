package vm

import (
	"reflect"
	"testing"

	v1 "kubevirt.io/api/core/v1"
)

// applyVM returns a VM with the given domain wrapped in a template.
func applyVM(d v1.DomainSpec) *v1.VirtualMachine {
	return &v1.VirtualMachine{
		Spec: v1.VirtualMachineSpec{
			Template: &v1.VirtualMachineInstanceTemplateSpec{
				Spec: v1.VirtualMachineInstanceSpec{Domain: d},
			},
		},
	}
}

func tpmInput(enabled, persistent *bool) *AdvancedOptionsInput {
	adv := &AdvancedOptionsInput{}
	adv.Devices.Tpm.Enabled = enabled
	adv.Devices.Tpm.Persistent = persistent
	return adv
}

func efiInput(enabled, secureBoot, persistent *bool) *AdvancedOptionsInput {
	adv := &AdvancedOptionsInput{}
	adv.Firmware.Bootloader.Efi.Enabled = enabled
	adv.Firmware.Bootloader.Efi.SecureBoot = secureBoot
	adv.Firmware.Bootloader.Efi.Persistent = persistent
	return adv
}

func TestWithAdvancedOptions(t *testing.T) {
	t.Run("nil block is a no-op", func(t *testing.T) {
		vm := applyVM(v1.DomainSpec{
			Devices:  v1.Devices{TPM: &v1.TPMDevice{Persistent: boolPtr(true)}},
			Firmware: &v1.Firmware{Bootloader: &v1.Bootloader{EFI: &v1.EFI{SecureBoot: boolPtr(true)}}},
		})
		before := vm.DeepCopy()

		withAdvancedOptions(vm, nil)

		if !reflect.DeepEqual(vm.Spec.Template.Spec.Domain, before.Spec.Template.Spec.Domain) {
			t.Fatalf("domain changed by nil advanced block: got %+v", vm.Spec.Template.Spec.Domain)
		}
	})

	t.Run("tpm disabled removes the device", func(t *testing.T) {
		vm := applyVM(v1.DomainSpec{})
		withAdvancedOptions(vm, tpmInput(boolPtr(false), nil))

		tpm := vm.Spec.Template.Spec.Domain.Devices.TPM
		if tpm == nil || tpm.Enabled == nil || *tpm.Enabled {
			t.Fatalf("expected tpm.enabled=false, got %+v", tpm)
		}
	})

	t.Run("tpm enabled non-persistent", func(t *testing.T) {
		vm := applyVM(v1.DomainSpec{})
		withAdvancedOptions(vm, tpmInput(boolPtr(true), boolPtr(false)))

		tpm := vm.Spec.Template.Spec.Domain.Devices.TPM
		if tpm == nil || tpm.Enabled == nil || !*tpm.Enabled {
			t.Fatalf("expected tpm.enabled=true, got %+v", tpm)
		}
		if tpm.Persistent == nil || *tpm.Persistent {
			t.Fatalf("expected tpm.persistent=false, got %+v", tpm)
		}
	})

	t.Run("tpm enabled with no sub-fields leaves persistent nil (kubevirt default)", func(t *testing.T) {
		vm := applyVM(v1.DomainSpec{})
		withAdvancedOptions(vm, tpmInput(boolPtr(true), nil))

		tpm := vm.Spec.Template.Spec.Domain.Devices.TPM
		if tpm == nil || tpm.Enabled == nil || !*tpm.Enabled {
			t.Fatalf("expected tpm.enabled=true, got %+v", tpm)
		}
		// No persistent given: leave it nil so kubevirt applies its default (false).
		if tpm.Persistent != nil {
			t.Fatalf("expected tpm.persistent left nil (kubevirt default), got %+v", tpm.Persistent)
		}
	})

	t.Run("inherit clears a prior tpm override", func(t *testing.T) {
		vm := applyVM(v1.DomainSpec{
			Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(false)}},
		})
		withAdvancedOptions(vm, tpmInput(nil, nil))

		if vm.Spec.Template.Spec.Domain.Devices.TPM != nil {
			t.Fatalf("expected tpm cleared to nil, got %+v", vm.Spec.Template.Spec.Domain.Devices.TPM)
		}
	})

	t.Run("secure boot true enables SMM", func(t *testing.T) {
		vm := applyVM(v1.DomainSpec{})
		adv := &AdvancedOptionsInput{}
		adv.Firmware.Bootloader.Efi.SecureBoot = boolPtr(true)
		withAdvancedOptions(vm, adv)

		efi := vm.Spec.Template.Spec.Domain.Firmware.Bootloader.EFI
		if efi == nil || efi.SecureBoot == nil || !*efi.SecureBoot {
			t.Fatalf("expected efi.secureBoot=true, got %+v", efi)
		}
		smm := vm.Spec.Template.Spec.Domain.Features.SMM
		if smm == nil || smm.Enabled == nil || !*smm.Enabled {
			t.Fatalf("expected features.smm.enabled=true, got %+v", smm)
		}
	})

	t.Run("secure boot false keeps EFI, clears SMM", func(t *testing.T) {
		vm := applyVM(v1.DomainSpec{
			Features: &v1.Features{SMM: &v1.FeatureState{Enabled: boolPtr(true)}},
		})
		adv := &AdvancedOptionsInput{}
		adv.Firmware.Bootloader.Efi.SecureBoot = boolPtr(false)
		withAdvancedOptions(vm, adv)

		efi := vm.Spec.Template.Spec.Domain.Firmware.Bootloader.EFI
		if efi == nil || efi.SecureBoot == nil || *efi.SecureBoot {
			t.Fatalf("expected efi.secureBoot=false, got %+v", efi)
		}
		if smm := vm.Spec.Template.Spec.Domain.Features.SMM; smm != nil {
			t.Fatalf("expected SMM cleared when secure boot is false, got %+v", smm)
		}
	})

	t.Run("persistent uefi, secure boot inherited, enables SMM and leaves secureBoot nil", func(t *testing.T) {
		vm := applyVM(v1.DomainSpec{})
		adv := &AdvancedOptionsInput{}
		adv.Firmware.Bootloader.Efi.Persistent = boolPtr(true)
		withAdvancedOptions(vm, adv)

		efi := vm.Spec.Template.Spec.Domain.Firmware.Bootloader.EFI
		if efi == nil || efi.Persistent == nil || !*efi.Persistent {
			t.Fatalf("expected efi.persistent=true, got %+v", efi)
		}
		// Secure boot is left to inherit (nil); kubevirt will default it to true
		// because the EFI block is present, so we must enable SMM to keep it valid.
		if efi.SecureBoot != nil {
			t.Fatalf("expected efi.secureBoot left nil (inherited), got %+v", efi.SecureBoot)
		}
		smm := vm.Spec.Template.Spec.Domain.Features.SMM
		if smm == nil || smm.Enabled == nil || !*smm.Enabled {
			t.Fatalf("expected features.smm.enabled=true, got %+v", smm)
		}
	})

	t.Run("full inherit drops the EFI override", func(t *testing.T) {
		vm := applyVM(v1.DomainSpec{
			Firmware: &v1.Firmware{Bootloader: &v1.Bootloader{EFI: &v1.EFI{Persistent: boolPtr(true)}}},
		})
		withAdvancedOptions(vm, &AdvancedOptionsInput{})

		fw := vm.Spec.Template.Spec.Domain.Firmware
		if fw != nil && fw.Bootloader != nil {
			t.Fatalf("expected EFI bootloader override dropped, got %+v", fw.Bootloader)
		}
	})

	t.Run("full inherit drops a prior secure boot override and SMM", func(t *testing.T) {
		vm := applyVM(v1.DomainSpec{
			Firmware: &v1.Firmware{Bootloader: &v1.Bootloader{EFI: &v1.EFI{SecureBoot: boolPtr(true)}}},
			Features: &v1.Features{SMM: &v1.FeatureState{Enabled: boolPtr(true)}},
		})
		withAdvancedOptions(vm, &AdvancedOptionsInput{})

		if fw := vm.Spec.Template.Spec.Domain.Firmware; fw != nil && fw.Bootloader != nil {
			t.Fatalf("expected EFI bootloader override dropped, got %+v", fw.Bootloader)
		}
		if smm := vm.Spec.Template.Spec.Domain.Features.SMM; smm != nil {
			t.Fatalf("expected SMM dropped on inherit, got %+v", smm)
		}
	})

	t.Run("full inherit drops a prior BIOS override", func(t *testing.T) {
		// Switching the EFI toggle back to auto from a prior "Off" (BIOS) must drop
		// the BIOS block too; a surviving BIOS keeps the VM pinned (read back as
		// source "vm") instead of inheriting the instance type's PreferredEfi.
		vm := applyVM(v1.DomainSpec{
			Firmware: &v1.Firmware{Bootloader: &v1.Bootloader{BIOS: &v1.BIOS{}}},
		})
		withAdvancedOptions(vm, &AdvancedOptionsInput{})

		if fw := vm.Spec.Template.Spec.Domain.Firmware; fw != nil && fw.Bootloader != nil {
			t.Fatalf("expected BIOS bootloader override dropped, got %+v", fw.Bootloader)
		}
	})

	t.Run("explicit all-null efi payload drops a prior BIOS override", func(t *testing.T) {
		// The exact payload the UI sends when moving from BIOS override to auto:
		// efi {enabled:null, secureBoot:null, persistent:null}.
		vm := applyVM(v1.DomainSpec{
			Firmware: &v1.Firmware{Bootloader: &v1.Bootloader{BIOS: &v1.BIOS{}}},
			Features: &v1.Features{SMM: &v1.FeatureState{Enabled: boolPtr(true)}},
		})
		withAdvancedOptions(vm, efiInput(nil, nil, nil))

		if fw := vm.Spec.Template.Spec.Domain.Firmware; fw != nil && fw.Bootloader != nil {
			t.Fatalf("expected bootloader override dropped, got %+v", fw.Bootloader)
		}
		if smm := vm.Spec.Template.Spec.Domain.Features.SMM; smm != nil {
			t.Fatalf("expected SMM dropped on inherit, got %+v", smm)
		}
	})

	t.Run("efi enabled with no sub-fields leaves sub-fields nil and enables SMM (kubevirt defaults)", func(t *testing.T) {
		vm := applyVM(v1.DomainSpec{})
		withAdvancedOptions(vm, efiInput(boolPtr(true), nil, nil))

		efi := vm.Spec.Template.Spec.Domain.Firmware.Bootloader.EFI
		if efi == nil || efi.SecureBoot != nil {
			t.Fatalf("expected EFI present with secureBoot nil (kubevirt default true), got %+v", efi)
		}
		// No persistent given: leave it nil so kubevirt applies its default (false).
		if efi.Persistent != nil {
			t.Fatalf("expected efi.persistent left nil (kubevirt default), got %+v", efi.Persistent)
		}
		// secureBoot nil defaults to true on kubevirt's side, which requires SMM.
		smm := vm.Spec.Template.Spec.Domain.Features.SMM
		if smm == nil || smm.Enabled == nil || !*smm.Enabled {
			t.Fatalf("expected features.smm.enabled=true, got %+v", smm)
		}
	})

	t.Run("efi off forces BIOS", func(t *testing.T) {
		vm := applyVM(v1.DomainSpec{})
		withAdvancedOptions(vm, efiInput(boolPtr(false), nil, nil))

		bl := vm.Spec.Template.Spec.Domain.Firmware.Bootloader
		if bl == nil || bl.BIOS == nil {
			t.Fatalf("expected BIOS bootloader set, got %+v", bl)
		}
		if bl.EFI != nil {
			t.Fatalf("expected no EFI block, got %+v", bl.EFI)
		}
	})

	t.Run("efi off from prior EFI+SMM forces BIOS and clears SMM", func(t *testing.T) {
		vm := applyVM(v1.DomainSpec{
			Firmware: &v1.Firmware{Bootloader: &v1.Bootloader{EFI: &v1.EFI{SecureBoot: boolPtr(true)}}},
			Features: &v1.Features{SMM: &v1.FeatureState{Enabled: boolPtr(true)}},
		})
		withAdvancedOptions(vm, efiInput(boolPtr(false), nil, nil))

		bl := vm.Spec.Template.Spec.Domain.Firmware.Bootloader
		if bl == nil || bl.BIOS == nil || bl.EFI != nil {
			t.Fatalf("expected BIOS set and EFI dropped, got %+v", bl)
		}
		if smm := vm.Spec.Template.Spec.Domain.Features.SMM; smm != nil {
			t.Fatalf("expected SMM cleared when forcing BIOS, got %+v", smm)
		}
	})

	t.Run("efi on from prior BIOS clears BIOS and sets EFI", func(t *testing.T) {
		// Re-enabling EFI after a prior "Off" (BIOS) must drop the BIOS block;
		// kubevirt rejects a bootloader carrying both EFI and BIOS.
		vm := applyVM(v1.DomainSpec{
			Firmware: &v1.Firmware{Bootloader: &v1.Bootloader{BIOS: &v1.BIOS{}}},
		})
		withAdvancedOptions(vm, efiInput(boolPtr(true), nil, nil))

		bl := vm.Spec.Template.Spec.Domain.Firmware.Bootloader
		if bl == nil || bl.EFI == nil {
			t.Fatalf("expected EFI bootloader set, got %+v", bl)
		}
		if bl.BIOS != nil {
			t.Fatalf("expected BIOS dropped when enabling EFI, got %+v", bl.BIOS)
		}
		smm := vm.Spec.Template.Spec.Domain.Features.SMM
		if smm == nil || smm.Enabled == nil || !*smm.Enabled {
			t.Fatalf("expected features.smm.enabled=true, got %+v", smm)
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		adv := tpmInput(boolPtr(true), boolPtr(true))
		adv.Firmware.Bootloader.Efi.SecureBoot = boolPtr(true)
		adv.Firmware.Bootloader.Efi.Persistent = boolPtr(true)

		vm1 := applyVM(v1.DomainSpec{})
		withAdvancedOptions(vm1, adv)

		vm2 := applyVM(v1.DomainSpec{})
		withAdvancedOptions(vm2, adv)
		withAdvancedOptions(vm2, adv)

		if !reflect.DeepEqual(vm1.Spec.Template.Spec.Domain, vm2.Spec.Template.Spec.Domain) {
			t.Fatalf("withAdvancedOptions is not idempotent:\nonce: %+v\ntwice: %+v",
				vm1.Spec.Template.Spec.Domain, vm2.Spec.Template.Spec.Domain)
		}
	})
}
