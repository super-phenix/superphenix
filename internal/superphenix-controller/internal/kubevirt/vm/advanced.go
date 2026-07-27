package vm

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/api/core/v1"
)

// GetAdvancedOptions resolves the effective advanced options of an existing VM.
// Effective values come from the expanded spec (instancetype + preference
// applied); the source is derived by comparing against the raw user spec.
func GetAdvancedOptions(ctx context.Context, namespace, name string) (view.AdvancedOptions, error) {
	log := logger.GetLogger(ctx)

	raw, err := config.VirtClient.VirtualMachine(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error getting VM for advanced options")
		return view.AdvancedOptions{}, err
	}
	if err := utils.CheckProjectLabel(raw, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("VM access denied")
		return view.AdvancedOptions{}, err
	}

	expanded, err := config.VirtClient.VirtualMachine(namespace).GetWithExpandedSpec(ctx, name)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error expanding VM spec for advanced options")
		return view.AdvancedOptions{}, err
	}

	// When the VM is running, the VMI carries the values applied at boot.
	// A NotFound means the VM is stopped (no live values).
	var vmi *v1.VirtualMachineInstance
	vmi, err = config.VirtClient.VirtualMachineInstance(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error getting VMI for advanced options")
			return view.AdvancedOptions{}, err
		}
		vmi = nil
	}

	return ResolveAdvancedOptions(raw, expanded, vmi), nil
}

// GetPreferenceAdvancedOptions resolves the advanced options a preference would
// apply, with no existing VM. It expands an in-memory stub VM carrying only the
// preference name, so sources are "preference" or "default". Used by the create
// form to show the defaults of the chosen vmType.
func GetPreferenceAdvancedOptions(ctx context.Context, namespace, prefName string) (view.AdvancedOptions, error) {
	log := logger.GetLogger(ctx)

	stub := &v1.VirtualMachine{
		TypeMeta: k8smetav1.TypeMeta{
			Kind:       v1.VirtualMachineGroupVersionKind.Kind,
			APIVersion: v1.VirtualMachineGroupVersionKind.GroupVersion().String(),
		},
		Spec: v1.VirtualMachineSpec{
			Preference: &v1.PreferenceMatcher{Name: prefName},
			Template:   &v1.VirtualMachineInstanceTemplateSpec{},
		},
	}

	expanded, err := config.VirtClient.ExpandSpec(namespace).ForVirtualMachine(stub)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("preference", prefName).Msg("Error expanding preference for advanced options")
		return view.AdvancedOptions{}, err
	}

	return ResolveAdvancedOptions(stub, expanded, nil), nil
}

// fieldSpec describes one advanced option as data: presentation metadata plus
// how it reads from a domain spec.
//   - value reads the option as a *bool (nil when unset).
//   - activeDefault is the value the option takes when its block is turned on
//     (e.g. Secure Boot becomes true once EFI is on).
//   - fallback is the value when the option is set nowhere (nil means false).
//     It's a function because Secure Boot's default depends on whether EFI is
//     present in the same domain.
type fieldSpec struct {
	key, label, description string
	value                   func(*v1.DomainSpec) *bool
	activeDefault           bool
	fallback                func(*v1.DomainSpec) bool
}

// blockSpec pairs a block's metadata (label/description, the input path, the
// on/off segment labels) with the enable toggle's field and its sub-fields.
type blockSpec struct {
	key, label, description, path, onLabel, offLabel string
	enabled                                          fieldSpec
	fields                                           []fieldSpec
}

// advancedSchema is the single source of truth for the advanced-options UI:
// which blocks/fields exist, their order, labels, descriptions, on/off labels,
// and how each reads from the VM. Adding an option here surfaces it in both the
// create and details views with no frontend change.
var advancedSchema = []blockSpec{
	{
		key:         "tpm",
		label:       "TPM",
		description: "Trusted Platform Module device",
		path:        "devices.tpm",
		onLabel:     "Enabled",
		offLabel:    "Disabled",
		enabled: fieldSpec{
			activeDefault: true,
			value: func(d *v1.DomainSpec) *bool {
				t := tpmOf(d)
				if t == nil {
					return nil
				}
				// kubevirt defaults a present TPM to on.
				return boolPtr(t.Enabled == nil || *t.Enabled)
			},
		},
		fields: []fieldSpec{
			{
				key:         "persistent",
				label:       "Persistent",
				description: "Persist the TPM state across reboots",
				value: func(d *v1.DomainSpec) *bool {
					if t := tpmOf(d); t != nil {
						return t.Persistent
					}
					return nil
				},
			},
		},
	},
	{
		key:         "efi",
		label:       "EFI firmware",
		description: "UEFI boot. Disabled boots legacy BIOS.",
		path:        "firmware.bootloader.efi",
		onLabel:     "EFI",
		offLabel:    "BIOS",
		// EFI block state: value=true means EFI is the active bootloader, false
		// means BIOS, nil means neither is set (kubevirt boots BIOS). The
		// activation default is true (turning the block on means EFI).
		enabled: fieldSpec{
			value:         bootloaderEfiActive,
			activeDefault: true,
		},
		fields: []fieldSpec{
			{
				key:         "secureBoot",
				label:       "Secure Boot",
				description: "UEFI Secure Boot (enables SMM)",
				// Secure Boot's effective default mirrors virt-launcher: true when an
				// EFI bootloader is present (set on the VM or induced by the
				// preference), false when there is no EFI at all. The expand-spec layer
				// never materializes the nil->true default, so we apply it via fallback.
				value: func(d *v1.DomainSpec) *bool {
					if e := efiOf(d); e != nil {
						return e.SecureBoot
					}
					return nil
				},
				activeDefault: true,
				fallback:      func(d *v1.DomainSpec) bool { return efiOf(d) != nil },
			},
			{
				key:         "persistent",
				label:       "EFI persistent",
				description: "Persist EFI NVRAM (VARS) across restarts",
				value: func(d *v1.DomainSpec) *bool {
					if e := efiOf(d); e != nil {
						return e.Persistent
					}
					return nil
				},
			},
		},
	},
}

// ResolveAdvancedOptions computes, for each option in advancedSchema, the
// effective value (from the expanded spec) and its source (from the raw spec):
//
//   - field set in raw      -> source "vm"
//   - set in expanded only  -> source "preference"
//   - absent from both      -> source "default" (with the kubevirt default value)
//
// When vmi is non-nil (VM running), each option also gets the value applied on
// the VMI (Live) and a Stale flag when that differs from Value. The result is an
// ordered list of blocks carrying both metadata and resolved state.
func ResolveAdvancedOptions(raw, expanded *v1.VirtualMachine, vmi *v1.VirtualMachineInstance) view.AdvancedOptions {
	rawDom, expDom, liveDom := domainOf(raw), domainOf(expanded), domainOfVMI(vmi)
	hasLive := vmi != nil

	out := view.AdvancedOptions{Blocks: make([]view.ResolvedBlock, 0, len(advancedSchema))}
	for _, bs := range advancedSchema {
		block := view.ResolvedBlock{
			Key:         bs.key,
			Label:       bs.label,
			Description: bs.description,
			Path:        bs.path,
			OnLabel:     bs.onLabel,
			OffLabel:    bs.offLabel,
			Enabled:     resolveField(bs.enabled, rawDom, expDom, liveDom, hasLive),
			Fields:      make([]view.ResolvedField, 0, len(bs.fields)),
		}
		for _, fs := range bs.fields {
			block.Fields = append(block.Fields, view.ResolvedField{
				Key:          fs.key,
				Label:        fs.label,
				Description:  fs.description,
				ResolvedBool: resolveField(fs, rawDom, expDom, liveDom, hasLive),
			})
		}
		out.Blocks = append(out.Blocks, block)
	}
	return out
}

// resolveField builds one option's ResolvedBool. The effective value/source
// prefers the raw (user-set) value, then the expanded (preference) value, then
// the field's kubevirt default. When the VM is running (hasLive), it also
// materializes the value applied on the VMI and flags Stale when it differs.
func resolveField(fs fieldSpec, raw, expanded, live *v1.DomainSpec, hasLive bool) view.ResolvedBool {
	fallback := func(d *v1.DomainSpec) bool {
		return fs.fallback != nil && fs.fallback(d)
	}

	var rb view.ResolvedBool
	switch {
	case fs.value(raw) != nil:
		rb = view.ResolvedBool{Value: boolPtr(*fs.value(raw)), Source: view.SourceVM}
	case fs.value(expanded) != nil:
		rb = view.ResolvedBool{Value: boolPtr(*fs.value(expanded)), Source: view.SourcePreference}
	default:
		rb = view.ResolvedBool{Value: boolPtr(fallback(expanded)), Source: view.SourceDefault}
	}
	rb.Default = fs.activeDefault

	if hasLive {
		lv := fallback(live)
		if v := fs.value(live); v != nil {
			lv = *v
		}
		rb.Live = &lv
		rb.Stale = rb.Value == nil || *rb.Value != lv
	}
	return rb
}

// nil-safe accessors. The VM template and VMI share the same DomainSpec shape
// (vm.Spec.Template.Spec.Domain vs vmi.Spec.Domain), so domainOf/domainOfVMI
// reduce both to a *v1.DomainSpec and the field accessors below serve both.

func domainOf(vm *v1.VirtualMachine) *v1.DomainSpec {
	if vm == nil || vm.Spec.Template == nil {
		return nil
	}
	return &vm.Spec.Template.Spec.Domain
}

func domainOfVMI(vmi *v1.VirtualMachineInstance) *v1.DomainSpec {
	if vmi == nil {
		return nil
	}
	return &vmi.Spec.Domain
}

func tpmOf(d *v1.DomainSpec) *v1.TPMDevice {
	if d == nil {
		return nil
	}
	return d.Devices.TPM
}

// efiOf walks domain -> firmware -> bootloader -> EFI, nil if any hop is unset.
func efiOf(d *v1.DomainSpec) *v1.EFI {
	if d == nil || d.Firmware == nil || d.Firmware.Bootloader == nil {
		return nil
	}
	return d.Firmware.Bootloader.EFI
}

// bootloaderEfiActive reports whether EFI is the active bootloader: true if an
// EFI block is present, false if BIOS is present, nil if neither is set.
func bootloaderEfiActive(d *v1.DomainSpec) *bool {
	if d == nil || d.Firmware == nil || d.Firmware.Bootloader == nil {
		return nil
	}
	switch {
	case d.Firmware.Bootloader.EFI != nil:
		return boolPtr(true)
	case d.Firmware.Bootloader.BIOS != nil:
		return boolPtr(false)
	default:
		return nil
	}
}

func boolPtr(b bool) *bool {
	return &b
}
