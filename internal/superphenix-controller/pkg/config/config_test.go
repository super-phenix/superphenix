package config

import (
	"maps"
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
	}{
		{
			name:            "defaults preserve key case",
			raw:             "azName: az1\n",
			wantAnnotations: defaultAnnotations,
			wantLabels:      defaultLabels,
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadTestDefaults(t)

			if err := loadTestUserConfig(t, tt.raw); err != nil {
				t.Fatalf("Unmarshal() returned an unexpected error: %v", err)
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
