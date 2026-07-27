package vm

import (
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"

	v1 "kubevirt.io/api/core/v1"
)

// TestAdvancedSchema locks the schema contract the frontend renders from: block
// order, keys, labels, descriptions, paths, on/off labels and field metadata.
// Changing it is deliberate (the UI shows whatever this emits).
func TestAdvancedSchema(t *testing.T) {
	blocks := ResolveAdvancedOptions(advDomainVM(v1.DomainSpec{}), advDomainVM(v1.DomainSpec{}), nil).Blocks

	type fieldMeta struct{ key, label, description string }
	type blockMeta struct {
		key, label, description, path, onLabel, offLabel string
		fields                                           []fieldMeta
	}
	want := []blockMeta{
		{
			key: "tpm", label: "TPM", description: "Trusted Platform Module device",
			path: "devices.tpm", onLabel: "Enabled", offLabel: "Disabled",
			fields: []fieldMeta{{"persistent", "Persistent", "Persist the TPM state across reboots"}},
		},
		{
			key: "efi", label: "EFI firmware", description: "UEFI boot. Disabled boots legacy BIOS.",
			path: "firmware.bootloader.efi", onLabel: "EFI", offLabel: "BIOS",
			fields: []fieldMeta{
				{"secureBoot", "Secure Boot", "UEFI Secure Boot (enables SMM)"},
				{"persistent", "EFI persistent", "Persist EFI NVRAM (VARS) across restarts"},
			},
		},
	}

	if len(blocks) != len(want) {
		t.Fatalf("block count: got %d, want %d", len(blocks), len(want))
	}
	for i, w := range want {
		b := blocks[i]
		if b.Key != w.key || b.Label != w.label || b.Description != w.description ||
			b.Path != w.path || b.OnLabel != w.onLabel || b.OffLabel != w.offLabel {
			t.Errorf("block %d metadata: got %+v, want %+v", i, b, w)
		}
		if len(b.Fields) != len(w.fields) {
			t.Fatalf("block %q field count: got %d, want %d", b.Key, len(b.Fields), len(w.fields))
		}
		for j, wf := range w.fields {
			f := b.Fields[j]
			if f.Key != wf.key || f.Label != wf.label || f.Description != wf.description {
				t.Errorf("block %q field %d: got {%q %q %q}, want {%q %q %q}",
					b.Key, j, f.Key, f.Label, f.Description, wf.key, wf.label, wf.description)
			}
		}
	}
}

// TestResolveAdvancedOptionsDefaults locks the kubevirt block-on defaults the UI
// seeds from: Secure Boot and the enable toggles default true, persistents false.
func TestResolveAdvancedOptionsDefaults(t *testing.T) {
	leaves := advLeaves(ResolveAdvancedOptions(advDomainVM(v1.DomainSpec{}), advDomainVM(v1.DomainSpec{}), nil))
	want := map[string]bool{
		"tpm.enabled": true, "tpm.persistent": false,
		"efi.enabled": true, "efi.secureBoot": true, "efi.persistent": false,
	}
	for k, w := range want {
		if got := leaves[k].Default; got != w {
			t.Errorf("%s default: got %v, want %v", k, got, w)
		}
	}
}

// advDomainVM wraps a DomainSpec into a minimal VirtualMachine.
func advDomainVM(d v1.DomainSpec) *v1.VirtualMachine {
	return &v1.VirtualMachine{
		Spec: v1.VirtualMachineSpec{
			Template: &v1.VirtualMachineInstanceTemplateSpec{
				Spec: v1.VirtualMachineInstanceSpec{Domain: d},
			},
		},
	}
}

// advDomainVMI wraps a DomainSpec into a minimal VirtualMachineInstance.
func advDomainVMI(d v1.DomainSpec) *v1.VirtualMachineInstance {
	return &v1.VirtualMachineInstance{
		Spec: v1.VirtualMachineInstanceSpec{Domain: d},
	}
}

func efiDomain(secureBoot *bool) v1.DomainSpec {
	return v1.DomainSpec{
		Firmware: &v1.Firmware{
			Bootloader: &v1.Bootloader{
				EFI: &v1.EFI{SecureBoot: secureBoot},
			},
		},
	}
}

func efiPersistentDomain(persistent *bool) v1.DomainSpec {
	return v1.DomainSpec{
		Firmware: &v1.Firmware{
			Bootloader: &v1.Bootloader{
				EFI: &v1.EFI{Persistent: persistent},
			},
		},
	}
}

// rbVal builds an expected ResolvedBool.
func rbVal(v bool, source string) view.ResolvedBool {
	return view.ResolvedBool{Value: boolPtr(v), Source: source}
}

// biosDomain wraps a VM/VMI domain that forces the BIOS bootloader.
func biosDomain() v1.DomainSpec {
	return v1.DomainSpec{
		Firmware: &v1.Firmware{Bootloader: &v1.Bootloader{BIOS: &v1.BIOS{}}},
	}
}

// advLeaves flattens the resolved blocks into a "<blockKey>.<fieldKey>" map
// (the enable toggle is "<blockKey>.enabled") for comparison.
func advLeaves(o view.AdvancedOptions) map[string]view.ResolvedBool {
	m := map[string]view.ResolvedBool{}
	for _, b := range o.Blocks {
		m[b.Key+".enabled"] = b.Enabled
		for _, f := range b.Fields {
			m[b.Key+"."+f.Key] = f.ResolvedBool
		}
	}
	return m
}

func eqBoolPtr(a, b *bool) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || *a == *b
}

func eqResolvedBool(a, b view.ResolvedBool) bool {
	if a.Source != b.Source {
		return false
	}
	if !eqBoolPtr(a.Value, b.Value) {
		return false
	}
	if !eqBoolPtr(a.Live, b.Live) {
		return false
	}
	if a.Stale != b.Stale {
		return false
	}
	return true
}

func TestResolveAdvancedOptions(t *testing.T) {
	tests := []struct {
		name     string
		raw      *v1.VirtualMachine
		expanded *v1.VirtualMachine
		vmi      *v1.VirtualMachineInstance // nil = VM not running (no live values)
		// want holds only the leaves the case asserts on.
		want map[string]view.ResolvedBool
	}{
		{
			name:     "all defaults when nothing is set",
			raw:      advDomainVM(v1.DomainSpec{}),
			expanded: advDomainVM(v1.DomainSpec{}),
			want: map[string]view.ResolvedBool{
				"tpm.enabled":    rbVal(false, view.SourceDefault),
				"tpm.persistent": rbVal(false, view.SourceDefault),
				"efi.enabled":    rbVal(false, view.SourceDefault),
				"efi.secureBoot": rbVal(false, view.SourceDefault),
				"efi.persistent": rbVal(false, view.SourceDefault),
			},
		},
		{
			name:     "efi enabled true from preference (instance type uses EFI)",
			raw:      advDomainVM(v1.DomainSpec{}),
			expanded: advDomainVM(efiDomain(boolPtr(true))),
			want: map[string]view.ResolvedBool{
				"efi.enabled": rbVal(true, view.SourcePreference),
			},
		},
		{
			name:     "efi enabled true forced on the vm",
			raw:      advDomainVM(efiDomain(boolPtr(true))),
			expanded: advDomainVM(efiDomain(boolPtr(true))),
			want: map[string]view.ResolvedBool{
				"efi.enabled": rbVal(true, view.SourceVM),
			},
		},
		{
			name:     "efi disabled (BIOS) forced on the vm",
			raw:      advDomainVM(biosDomain()),
			expanded: advDomainVM(biosDomain()),
			want: map[string]view.ResolvedBool{
				"efi.enabled": rbVal(false, view.SourceVM),
			},
		},
		{
			name: "tpm applied by preference (windows.2k25.virtio)",
			raw:  advDomainVM(v1.DomainSpec{}),
			expanded: advDomainVM(v1.DomainSpec{
				Devices: v1.Devices{TPM: &v1.TPMDevice{Persistent: boolPtr(true)}},
			}),
			want: map[string]view.ResolvedBool{
				"tpm.enabled":    rbVal(true, view.SourcePreference),
				"tpm.persistent": rbVal(true, view.SourcePreference),
			},
		},
		{
			name: "tpm removed on the vm overrides the preference",
			raw: advDomainVM(v1.DomainSpec{
				Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(false)}},
			}),
			expanded: advDomainVM(v1.DomainSpec{
				Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(false)}},
			}),
			want: map[string]view.ResolvedBool{
				"tpm.enabled": rbVal(false, view.SourceVM),
			},
		},
		{
			name: "tpm persistent forced false on the vm",
			raw: advDomainVM(v1.DomainSpec{
				Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(true), Persistent: boolPtr(false)}},
			}),
			expanded: advDomainVM(v1.DomainSpec{
				Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(true), Persistent: boolPtr(false)}},
			}),
			want: map[string]view.ResolvedBool{
				"tpm.enabled":    rbVal(true, view.SourceVM),
				"tpm.persistent": rbVal(false, view.SourceVM),
			},
		},
		{
			name:     "secure boot applied by preference",
			raw:      advDomainVM(v1.DomainSpec{}),
			expanded: advDomainVM(efiDomain(boolPtr(true))),
			want: map[string]view.ResolvedBool{
				"efi.secureBoot": rbVal(true, view.SourcePreference),
			},
		},
		{
			name:     "secure boot forced on the vm",
			raw:      advDomainVM(efiDomain(boolPtr(true))),
			expanded: advDomainVM(efiDomain(boolPtr(true))),
			want: map[string]view.ResolvedBool{
				"efi.secureBoot": rbVal(true, view.SourceVM),
			},
		},
		{
			name:     "persistent uefi forced on the vm",
			raw:      advDomainVM(efiPersistentDomain(boolPtr(true))),
			expanded: advDomainVM(efiPersistentDomain(boolPtr(true))),
			want: map[string]view.ResolvedBool{
				"efi.persistent": rbVal(true, view.SourceVM),
			},
		},
		{
			name:     "persistent uefi applied by preference",
			raw:      advDomainVM(v1.DomainSpec{}),
			expanded: advDomainVM(efiPersistentDomain(boolPtr(true))),
			want: map[string]view.ResolvedBool{
				"efi.persistent": rbVal(true, view.SourcePreference),
			},
		},
		{
			// The preference induces an EFI bootloader (here via persistent) but
			// leaves secureBoot unset; kubevirt defaults it to true, so the
			// effective value is true even though expand-spec keeps it nil.
			name:     "secure boot defaults true when an EFI bootloader is present",
			raw:      advDomainVM(v1.DomainSpec{}),
			expanded: advDomainVM(efiPersistentDomain(boolPtr(true))),
			want: map[string]view.ResolvedBool{
				"efi.secureBoot": rbVal(true, view.SourceDefault),
				"efi.persistent": rbVal(true, view.SourcePreference),
			},
		},
		{
			name:     "vm not running leaves live/stale unset",
			raw:      advDomainVM(v1.DomainSpec{}),
			expanded: advDomainVM(v1.DomainSpec{}),
			vmi:      nil,
			want: map[string]view.ResolvedBool{
				"tpm.enabled":    {Value: boolPtr(false), Source: view.SourceDefault},
				"tpm.persistent": {Value: boolPtr(false), Source: view.SourceDefault},
				"efi.secureBoot": {Value: boolPtr(false), Source: view.SourceDefault},
				"efi.persistent": {Value: boolPtr(false), Source: view.SourceDefault},
			},
		},
		{
			name: "tpm persistent changed to false while running is stale",
			raw: advDomainVM(v1.DomainSpec{
				Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(true), Persistent: boolPtr(false)}},
			}),
			expanded: advDomainVM(v1.DomainSpec{
				Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(true), Persistent: boolPtr(false)}},
			}),
			// VMI booted with the preference-applied persistent TPM.
			vmi: advDomainVMI(v1.DomainSpec{
				Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(true), Persistent: boolPtr(true)}},
			}),
			want: map[string]view.ResolvedBool{
				"tpm.persistent": {Value: boolPtr(false), Source: view.SourceVM, Live: boolPtr(true), Stale: true},
				"tpm.enabled":    {Value: boolPtr(true), Source: view.SourceVM, Live: boolPtr(true), Stale: false},
			},
		},
		{
			name: "running value matches saved value is not stale",
			raw: advDomainVM(v1.DomainSpec{
				Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(true)}},
			}),
			expanded: advDomainVM(v1.DomainSpec{
				Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(true)}},
			}),
			vmi: advDomainVMI(v1.DomainSpec{
				Devices: v1.Devices{TPM: &v1.TPMDevice{Enabled: boolPtr(true)}},
			}),
			want: map[string]view.ResolvedBool{
				"tpm.enabled": {Value: boolPtr(true), Source: view.SourceVM, Live: boolPtr(true), Stale: false},
			},
		},
		{
			name:     "persistent uefi on live but default on saved is stale",
			raw:      advDomainVM(v1.DomainSpec{}),
			expanded: advDomainVM(v1.DomainSpec{}),
			// Saved resolves to default false; the running VMI booted with it on.
			// The EFI block also makes live secure boot default to true.
			vmi: advDomainVMI(efiPersistentDomain(boolPtr(true))),
			want: map[string]view.ResolvedBool{
				"efi.persistent": {Value: boolPtr(false), Source: view.SourceDefault, Live: boolPtr(true), Stale: true},
				"efi.secureBoot": {Value: boolPtr(false), Source: view.SourceDefault, Live: boolPtr(true), Stale: true},
			},
		},
		{
			name:     "secure boot enabled on live but disabled on saved is stale",
			raw:      advDomainVM(v1.DomainSpec{}),
			expanded: advDomainVM(v1.DomainSpec{}),
			vmi:      advDomainVMI(efiDomain(boolPtr(true))),
			want: map[string]view.ResolvedBool{
				"efi.secureBoot": {Value: boolPtr(false), Source: view.SourceDefault, Live: boolPtr(true), Stale: true},
			},
		},
		{
			name:     "secure boot live matches saved default is not stale",
			raw:      advDomainVM(v1.DomainSpec{}),
			expanded: advDomainVM(v1.DomainSpec{}),
			// VMI has no EFI block -> resolves to default false, matching saved.
			vmi: advDomainVMI(v1.DomainSpec{}),
			want: map[string]view.ResolvedBool{
				"efi.secureBoot": {Value: boolPtr(false), Source: view.SourceDefault, Live: boolPtr(false), Stale: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := advLeaves(ResolveAdvancedOptions(tt.raw, tt.expanded, tt.vmi))
			for key, want := range tt.want {
				if !eqResolvedBool(got[key], want) {
					t.Errorf("%s: got %+v, want %+v", key, got[key], want)
				}
			}
		})
	}
}
