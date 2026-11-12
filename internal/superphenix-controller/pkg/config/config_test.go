package config

import (
	"maps"
	"reflect"
	"strings"
	"testing"
)

// loadTestDefaults resets Global and loads the hardcoded defaults.
func loadTestDefaults(t *testing.T) {
	t.Helper()

	Global = Config{}
	v.SetConfigType("yaml")
	if err := loadDefaults(); err != nil {
		t.Fatalf("loadDefaults() returned an unexpected error: %v", err)
	}
}

// loadTestUserConfig loads raw as the user supplied config.yaml.
func loadTestUserConfig(t *testing.T, raw string) error {
	t.Helper()

	if err := v.ReadConfig(strings.NewReader(raw)); err != nil {
		t.Fatalf("ReadConfig() returned an unexpected error: %v", err)
	}
	return v.Unmarshal(&Global)
}

// TestConfigCaseSensitiveKeys guards against viper lowercasing the keys of
// the annotation and label entry lists.
func TestConfigCaseSensitiveKeys(t *testing.T) {
	defaultAnnotations := map[string]string{
		"v1.multus-cni.io/default-network":   "kube-system/system-isolated-egress",
		"cdi.kubevirt.io/allowClaimAdoption": "true",
	}
	defaultLabels := map[string]string{
		"app.kubernetes.io/name": "sfs-kaas",
	}

	tests := []struct {
		name            string
		raw             string
		wantAnnotations map[string]string
		wantLabels      map[string]string
		wantMtu         int
		wantMtuAuto     bool
	}{
		{
			name:            "defaults preserve key case",
			raw:             "azName: az1\n",
			wantAnnotations: defaultAnnotations,
			wantLabels:      defaultLabels,
			wantMtuAuto:     true,
		},
		{
			name: "user annotations preserve key case and replace the defaults",
			raw: `
productsConfig:
  datavolume:
    defaultAnnotations:
      - key: "my.custom/Annotation"
        value: "yes"
`,
			wantAnnotations: map[string]string{"my.custom/Annotation": "yes"},
			wantLabels:      defaultLabels,
			wantMtuAuto:     true,
		},
		{
			name: "user labels preserve key case and replace the defaults",
			raw: `
disableEditionForResourcesByLabels:
  - key: "superphenix.net/managedBy"
    value: "operator"
`,
			wantAnnotations: defaultAnnotations,
			wantLabels:      map[string]string{"superphenix.net/managedBy": "operator"},
			wantMtuAuto:     true,
		},
		{
			name: "user mtu override",
			raw: `
productsConfig:
  subnets:
    mtu: 1400
    mtuAutodetection: false
`,
			wantAnnotations: defaultAnnotations,
			wantLabels:      defaultLabels,
			wantMtu:         1400,
			wantMtuAuto:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadTestDefaults(t)

			if tt.wantMtu == 0 {
				tt.wantMtu = 1500
			}

			if err := loadTestUserConfig(t, tt.raw); err != nil {
				t.Fatalf("Unmarshal() returned an unexpected error: %v", err)
			}

			if got := Global.ProductsConfig.Subnets.Mtu; got != tt.wantMtu {
				t.Errorf("mtu = %d, want %d", got, tt.wantMtu)
			}
			if got := Global.ProductsConfig.Subnets.MtuAutodetection; got != tt.wantMtuAuto {
				t.Errorf("mtuAutodetection = %v, want %v", got, tt.wantMtuAuto)
			}

			if got := Global.ProductsConfig.Datavolume.DefaultAnnotations.Map(); !maps.Equal(got, tt.wantAnnotations) {
				t.Errorf("datavolume annotations = %v, want %v", got, tt.wantAnnotations)
			}
			if got := Global.DisableEditionForResourcesByLabels.Map(); !maps.Equal(got, tt.wantLabels) {
				t.Errorf("disableEditionForResourcesByLabels = %v, want %v", got, tt.wantLabels)
			}
		})
	}
}

func TestDeviceMappingLoad(t *testing.T) {
	const (
		rtxDevice = "nvidia.com/GB202GL_RTX_PRO_6000_BLACKWELL_SERVER_EDITION"
		rtxClass  = "nvidia-rtx-pro-6000-bse"
		lowered   = "nvidia-rtx"
	)

	tests := []struct {
		name string
		raw  string
		want map[string]GpuClassEntry
	}{
		{
			name: "defaults to an empty mapping",
			raw:  "azName: az1\n",
			want: nil,
		},
		{
			name: "entry keeps device name case and fills the id",
			raw: `
productsConfig:
  compute:
    deviceMapping:
      nvidia-rtx-pro-6000-bse:
        deviceName: "` + rtxDevice + `"
        displayName: "NVIDIA RTX PRO 6000 Blackwell Server Edition"
`,
			want: map[string]GpuClassEntry{
				rtxClass: {ID: rtxClass, DisplayName: "NVIDIA RTX PRO 6000 Blackwell Server Edition", DeviceName: rtxDevice},
			},
		},
		{
			name: "empty display name falls back to the id",
			raw: `
productsConfig:
  compute:
    deviceMapping:
      nvidia-rtx-pro-6000-bse:
        deviceName: "` + rtxDevice + `"
`,
			want: map[string]GpuClassEntry{
				rtxClass: {ID: rtxClass, DisplayName: rtxClass, DeviceName: rtxDevice},
			},
		},
		{
			name: "entry without device name is dropped",
			raw: `
productsConfig:
  compute:
    deviceMapping:
      broken:
        displayName: "Broken"
      nvidia-rtx-pro-6000-bse:
        deviceName: "` + rtxDevice + `"
`,
			want: map[string]GpuClassEntry{
				rtxClass: {ID: rtxClass, DisplayName: rtxClass, DeviceName: rtxDevice},
			},
		},
		{
			name: "class keys are lowercased by the loader",
			raw: `
productsConfig:
  compute:
    deviceMapping:
      NVIDIA-RTX:
        deviceName: "` + rtxDevice + `"
`,
			want: map[string]GpuClassEntry{
				lowered: {ID: lowered, DisplayName: lowered, DeviceName: rtxDevice},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadTestDefaults(t)
			if err := loadTestUserConfig(t, tt.raw); err != nil {
				t.Fatalf("Unmarshal() returned an unexpected error: %v", err)
			}
			populateCatalogIDs()

			got := Global.ProductsConfig.Compute.DeviceMapping
			if len(got) == 0 && len(tt.want) == 0 {
				return // viper may decode "{}" as nil; both mean no GPU class
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("deviceMapping = %#v, want %#v", got, tt.want)
			}
		})
	}
}
