package view

// ResolvedBool is the resolved state of one advanced (device/firmware) option:
// the effective value and where it comes from. When the VM is running, Live is
// the value applied on the VMI and Stale is true when it differs from Value
// (a restart is needed to apply the saved value).
type ResolvedBool struct {
	Value   *bool  `json:"value"`           // effective value, nil if not set anywhere
	Source  string `json:"source"`          // "vm", "preference" or "default"
	Default bool   `json:"default"`         // kubevirt default this field takes when its block is active (e.g. Secure Boot is true once EFI is on)
	Live    *bool  `json:"live,omitempty"`  // value on the running VMI, nil if stopped
	Stale   bool   `json:"stale,omitempty"` // Live differs from Value (restart required)
}

const (
	SourceVM         = "vm"
	SourcePreference = "preference"
	SourceDefault    = "default"
)

// ResolvedField is one advanced option: its presentation metadata (so the UI
// renders labels/descriptions from the API, not hardcoded) plus its resolved
// state. Key matches the option's leaf name in AdvancedOptionsInput.
type ResolvedField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	ResolvedBool
}

// ResolvedBlock is a group of advanced options gated by an enable toggle
// (Auto/On/Off). Path is the dotted location of this block in
// AdvancedOptionsInput (e.g. "devices.tpm", "firmware.bootloader.efi") so the UI
// can build the typed payload generically. Enabled carries the toggle state;
// OnLabel/OffLabel name its on/off segments (Enabled/Disabled, EFI/BIOS).
type ResolvedBlock struct {
	Key         string          `json:"key"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
	Path        string          `json:"path"`
	OnLabel     string          `json:"onLabel"`
	OffLabel    string          `json:"offLabel"`
	Enabled     ResolvedBool    `json:"enabled"`
	Fields      []ResolvedField `json:"fields"`
}

// AdvancedOptions is the resolved advanced-options schema: an ordered list of
// blocks carrying both their presentation metadata and resolved values. The
// frontend renders it generically, so new options are added backend-side only.
type AdvancedOptions struct {
	Blocks []ResolvedBlock `json:"blocks"`
}
