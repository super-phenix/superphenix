package config

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestValidate(t *testing.T) {
	// A configuration that passes every rule; individual sub-tests mutate one
	// field at a time to exercise each rejection path.
	good := func() Config {
		var c Config
		c.Authentication.JwtIssuer = "superphenix-api"
		c.Authentication.JwtSecret = strings.Repeat("x", 32)
		c.Session.Cors.AllowedOrigins = []string{"https://app.example.com"}
		c.AuditLog.Enabled = true
		c.AuditLog.Retention.DefaultDays = 90
		c.AuditLog.Retention.MinDays = 1
		c.AuditLog.Retention.MaxDays = 365
		c.AuditLog.Retention.UserDays = 90
		c.AuditLog.GarbageCollection.Enabled = true
		c.AuditLog.GarbageCollection.Interval = time.Hour
		c.AuditLog.GarbageCollection.Timeout = 10 * time.Minute
		c.AuditLog.GarbageCollection.BatchSize = 5000
		return c
	}

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{name: "valid config", mutate: func(c *Config) {}, wantErr: ""},
		{name: "empty jwt secret", mutate: func(c *Config) { c.Authentication.JwtSecret = "" }, wantErr: "jwtSecret must not be empty"},
		{name: "default jwt secret", mutate: func(c *Config) { c.Authentication.JwtSecret = "secret" }, wantErr: `must not be the built-in default "secret"`},
		{name: "short jwt secret", mutate: func(c *Config) { c.Authentication.JwtSecret = "too-short" }, wantErr: "at least 32 bytes"},
		{name: "empty issuer", mutate: func(c *Config) { c.Authentication.JwtIssuer = "" }, wantErr: "jwtIssuer must not be empty"},
		{name: "wildcard origin without opt-in", mutate: func(c *Config) { c.Session.Cors.AllowedOrigins = []string{"https://app.example.com", "*"} }, wantErr: `set session.cors.allowUnsafeWildcard=true`},
		{name: "wildcard origin with opt-in accepted", mutate: func(c *Config) {
			c.Session.Cors.AllowedOrigins = []string{"https://app.example.com", "*"}
			c.Session.Cors.AllowUnsafeWildcard = true
		}, wantErr: ""},
		{name: "audit log disabled skips its rules", mutate: func(c *Config) { c.AuditLog = AuditLogConfig{} }, wantErr: ""},
		{name: "audit retention min below one", mutate: func(c *Config) { c.AuditLog.Retention.MinDays = 0 }, wantErr: "minDays <= defaultDays <= maxDays"},
		{name: "audit retention default above max", mutate: func(c *Config) { c.AuditLog.Retention.DefaultDays = 400 }, wantErr: "minDays <= defaultDays <= maxDays"},
		{name: "audit user retention below one", mutate: func(c *Config) { c.AuditLog.Retention.UserDays = 0 }, wantErr: "userDays must be at least 1"},
		{name: "audit gc without interval", mutate: func(c *Config) { c.AuditLog.GarbageCollection.Interval = 0 }, wantErr: "interval and timeout must be positive"},
		{name: "audit gc batch too large", mutate: func(c *Config) { c.AuditLog.GarbageCollection.BatchSize = 50001 }, wantErr: "batchSize must be between"},
		{name: "audit gc disabled skips its rules", mutate: func(c *Config) {
			c.AuditLog.GarbageCollection.Enabled = false
			c.AuditLog.GarbageCollection.BatchSize = 0
		}, wantErr: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := good()
			tt.mutate(&c)
			err := c.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() returned unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() returned nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestResolveKubeVersionRepo(t *testing.T) {
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
			got, supported := ResolveKubeVersionRepo(versions, def, tt.version)
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
			v := viper.New()
			v.SetConfigType("yaml")
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
