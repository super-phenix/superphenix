package config

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	// A configuration that passes every rule; individual sub-tests mutate one
	// field at a time to exercise each rejection path.
	good := func() Config {
		var c Config
		c.Authentication.JwtIssuer = "superphenix-api"
		c.Authentication.JwtSecret = strings.Repeat("x", 32)
		c.Session.Cors.AllowedOrigins = []string{"https://app.example.com"}
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
