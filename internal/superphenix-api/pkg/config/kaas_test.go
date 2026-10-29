package config

import (
	"bytes"
	"testing"
)

// TestKaasVersionsDecode checks the kubernetes.versions keys survive viper as
// single map keys and stay overridable by env variables.
func TestKaasVersionsDecode(t *testing.T) {
	const yaml = `argoController:
  appProjectNamespace: self-service-argocd
productsConfig:
  defaultSubnet:
    ipv6: "fd00:10:10::/64"
  argoApp:
    kubernetes:
      versions:
        "0.8.0":
          repo:
            repoURL: ghcr.io/super-phenix/charts
            chart: sfs-kaas
            targetRevision: "0.8.0"
          kubeVersions:
            - version: v1.37.0
            - version: v1.34.8
              fqdn: false
        "0.0.0":
          repo:
            repoURL: ghcr.io/super-phenix/charts
            chart: sfs-kaas
            targetRevision: "0.3.8"
          kubeVersions:
            - version: v1.34.8
`

	tests := []struct {
		name  string
		env   map[string]string
		check func(t *testing.T, c Config)
	}{
		{
			name: "version keys are kept whole",
			check: func(t *testing.T, c Config) {
				versions := c.ProductsConfig.ArgoApp.Kubernetes.Versions
				if len(versions) != 2 {
					t.Fatalf("Versions = %+v, want the 0.8.0 and 0.0.0 keys", versions)
				}
				p := versions["0.8.0"]
				if p.Repo.TargetRevision != "0.8.0" || len(p.KubeVersions) != 2 || p.KubeVersions[1].Fqdn == nil || *p.KubeVersions[1].Fqdn {
					t.Errorf(`Versions["0.8.0"] = %+v`, p)
				}
			},
		},
		{
			name: "IPv6 values are untouched",
			check: func(t *testing.T, c Config) {
				if got := c.ProductsConfig.DefaultSubnet.IPv6; got != "fd00:10:10::/64" {
					t.Errorf("DefaultSubnet.IPv6 = %q", got)
				}
			},
		},
		{
			name: "env variable overrides a nested value",
			env:  map[string]string{"SUPERPHENIX-API_ARGOCONTROLLER_APPPROJECTNAMESPACE": "from-env"},
			check: func(t *testing.T, c Config) {
				if got := c.ArgoController.AppProjectNamespace; got != "from-env" {
					t.Errorf("ArgoController.AppProjectNamespace = %q, want from-env", got)
				}
			},
		},
		{
			name: "env variable overrides a profile value",
			env:  map[string]string{"SUPERPHENIX-API_PRODUCTSCONFIG_ARGOAPP_KUBERNETES_VERSIONS_0.8.0_REPO_TARGETREVISION": "0.8.1"},
			check: func(t *testing.T, c Config) {
				if got := c.ProductsConfig.ArgoApp.Kubernetes.Versions["0.8.0"].Repo.TargetRevision; got != "0.8.1" {
					t.Errorf(`Versions["0.8.0"].Repo.TargetRevision = %q, want 0.8.1`, got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			v := newViper()
			if err := v.ReadConfig(bytes.NewBufferString(yaml)); err != nil {
				t.Fatalf("ReadConfig() error = %v", err)
			}
			var c Config
			if err := v.Unmarshal(&c); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			tt.check(t, c)
		})
	}
}

func TestCheckRemovedKeys(t *testing.T) {
	const versions = `productsConfig:
  argoApp:
    kubernetes:
      versions:
        "0.0.0":
          repo: {repoURL: ghcr.io/super-phenix/charts, chart: sfs-kaas, targetRevision: "0.6.3"}
          kubeVersions:
            - version: v1.36.3
`

	tests := []struct {
		name    string
		yaml    string
		env     map[string]string
		wantErr bool
	}{
		{name: "repo and kubeVersions inside a profile", yaml: versions},
		{name: "no kubernetes config", yaml: "spxPrefix: spx\n"},
		{name: "legacy repo", yaml: versions + "      repo: {repoURL: ghcr.io/super-phenix/charts, chart: sfs-kaas, targetRevision: \"0.7.0\"}\n", wantErr: true},
		{name: "legacy kubeVersions", yaml: versions + "      kubeVersions:\n        - version: v1.36.3\n", wantErr: true},
		{name: "legacy repo env variable", yaml: versions, env: map[string]string{"SUPERPHENIX-API_PRODUCTSCONFIG_ARGOAPP_KUBERNETES_REPO_TARGETREVISION": "0.7.0"}, wantErr: true},
		{name: "legacy kubeVersions env variable", yaml: versions, env: map[string]string{"SUPERPHENIX-API_PRODUCTSCONFIG_ARGOAPP_KUBERNETES_KUBEVERSIONS": "v1.36.3"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			v := newViper()
			if err := v.ReadConfig(bytes.NewBufferString(tt.yaml)); err != nil {
				t.Fatalf("ReadConfig() error = %v", err)
			}
			if err := checkRemovedKeys(v); (err != nil) != tt.wantErr {
				t.Errorf("checkRemovedKeys() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestKaasVersionProfileChart(t *testing.T) {
	def := RepoArgoAppConfig{
		RepoURL:        "ghcr.io/super-phenix/charts",
		TargetRevision: "0.1.0",
		Chart:          "sfs-kaas",
	}
	override := RepoArgoAppConfig{
		RepoURL:        "ghcr.io/super-phenix/edge",
		TargetRevision: "0.2.0",
		Chart:          "sfs-kaas-edge",
	}
	versions := []KubeVersionConfig{
		{Version: "v1.35.5", Repo: &override},
		{Version: "v1.34.8"},
	}

	tests := []struct {
		name      string
		version   string
		want      RepoArgoAppConfig
		supported bool
	}{
		{
			name:      "version without repo returns default",
			version:   "v1.34.8",
			want:      def,
			supported: true,
		},
		{
			name:      "version with repo returns override verbatim",
			version:   "v1.35.5",
			want:      override,
			supported: true,
		},
		{
			name:      "unknown version returns zero value and false",
			version:   "v1.0.0",
			want:      RepoArgoAppConfig{},
			supported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, supported := KaasVersionProfile{Repo: def, KubeVersions: versions}.Chart(tt.version)
			if supported != tt.supported {
				t.Fatalf("supported = %v, want %v", supported, tt.supported)
			}
			if got != tt.want {
				t.Errorf("repo = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestKaasAzDomain(t *testing.T) {
	azDomains := map[string]any{
		"aq01": "example.org",
		"aq01-test01": map[string]any{
			"internal": "azs.aq01.example.org",
			"external": "%s.kaas.aq01-test01.example.org",
		},
		"aq01-test02": map[string]any{"external": "%s.kaas.aq01-test02.example.org"},
	}

	tests := []struct {
		name   string
		az     string
		want   AzDomainConfig
		wantOk bool
	}{
		{name: "object entry", az: "aq01-test01", want: AzDomainConfig{Internal: "azs.aq01.example.org", External: "%s.kaas.aq01-test01.example.org"}, wantOk: true},
		{name: "partial object entry", az: "aq01-test02", want: AzDomainConfig{External: "%s.kaas.aq01-test02.example.org"}, wantOk: true},
		{name: "legacy string entry", az: "aq01", wantOk: false},
		{name: "missing key", az: "zz99-test01", wantOk: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Config
			c.ProductsConfig.ArgoApp.Kubernetes.AzDomains = azDomains
			got, ok := c.KaasAzDomain(tt.az)
			if ok != tt.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("KaasAzDomain(%q) = %+v, want %+v", tt.az, got, tt.want)
			}
		})
	}
}

// TestKaasAzDomainDecode checks the mixed azDomains shape loads through viper.
func TestKaasAzDomainDecode(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		az   string
		want AzDomainConfig
	}{
		{
			name: "mixed legacy and object entries",
			yaml: `productsConfig:
  argoApp:
    kubernetes:
      azDomains:
        aq01: example.org
        aq01-test01:
          internal: azs.aq01.example.org
          external: "%s.kaas.aq01-test01.example.org"
`,
			az:   "aq01-test01",
			want: AzDomainConfig{Internal: "azs.aq01.example.org", External: "%s.kaas.aq01-test01.example.org"},
		},
		{
			name: "object entries only",
			yaml: `productsConfig:
  argoApp:
    kubernetes:
      azDomains:
        aq01-test01:
          internal: azs.aq01.example.org
          external: "%s.kaas.aq01-test01.example.org"
`,
			az:   "aq01-test01",
			want: AzDomainConfig{Internal: "azs.aq01.example.org", External: "%s.kaas.aq01-test01.example.org"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newViper()
			if err := v.ReadConfig(bytes.NewBufferString(tt.yaml)); err != nil {
				t.Fatalf("ReadConfig() error = %v", err)
			}
			var c Config
			if err := v.Unmarshal(&c); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			got, ok := c.KaasAzDomain(tt.az)
			if !ok {
				t.Fatalf("KaasAzDomain(%q) not found in %+v", tt.az, c.ProductsConfig.ArgoApp.Kubernetes.AzDomains)
			}
			if got != tt.want {
				t.Errorf("KaasAzDomain(%q) = %+v, want %+v", tt.az, got, tt.want)
			}
		})
	}
}
