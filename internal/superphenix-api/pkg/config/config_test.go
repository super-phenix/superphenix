package config

import (
	"errors"
	"strings"
	"testing"
	"time"
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
		c.Operator.Namespace = "superphenix-system"
		c.ProductsConfig.ArgoApp.Kubernetes.Versions = VersionedProfiles[KaasVersionProfile]{
			"0.0.0": {Repo: testRepo("0.3.8"), KubeVersions: []KubeVersionConfig{{Version: "v1.34.8"}}},
		}
		return c
	}
	withVersions := func(versions VersionedProfiles[KaasVersionProfile]) func(*Config) {
		return func(c *Config) { c.ProductsConfig.ArgoApp.Kubernetes.Versions = versions }
	}
	validProfile := KaasVersionProfile{Repo: testRepo("0.8.0"), KubeVersions: []KubeVersionConfig{{Version: "v1.37.0"}}}

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
		{name: "empty operator namespace", mutate: func(c *Config) { c.Operator.Namespace = "" }, wantErr: "operator.namespace must not be empty"},
		{name: "no kaas versions", mutate: withVersions(nil), wantErr: "kubernetes.versions must not be empty"},
		{name: "kaas version key not semver", mutate: withVersions(VersionedProfiles[KaasVersionProfile]{"latest": validProfile}), wantErr: `versions["latest"]: key must be a semantic version`},
		{name: "kaas version keys normalised to the same version", mutate: withVersions(VersionedProfiles[KaasVersionProfile]{"0.8.0": validProfile, "v0.8.0": validProfile}), wantErr: "same version as"},
		{name: "kaas profile without repo", mutate: withVersions(VersionedProfiles[KaasVersionProfile]{"0.8.0": {KubeVersions: validProfile.KubeVersions}}), wantErr: `versions["0.8.0"].repo must set repoURL and targetRevision`},
		{name: "kaas profile without kube versions", mutate: withVersions(VersionedProfiles[KaasVersionProfile]{"0.8.0": {Repo: validProfile.Repo}}), wantErr: `versions["0.8.0"].kubeVersions must not be empty`},
		{name: "several kaas profiles", mutate: withVersions(VersionedProfiles[KaasVersionProfile]{"0.0.0": validProfile, "0.7.0": validProfile, "0.8.0": validProfile}), wantErr: ""},
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

func testRepo(revision string) RepoArgoAppConfig {
	return RepoArgoAppConfig{RepoURL: "ghcr.io/super-phenix/charts", Chart: "sfs-kaas", TargetRevision: revision}
}

func TestVersionedProfilesFor(t *testing.T) {
	const (
		latest   = "0.8.0 profile"
		previous = "0.7.0 profile"
		catchAll = "catch-all profile"
	)
	withCatchAll := VersionedProfiles[string]{
		"0.8.0": latest,
		"0.0.0": catchAll,
		"0.7.0": previous,
	}
	withoutCatchAll := VersionedProfiles[string]{
		"0.7.0": previous,
		"0.8.0": latest,
	}

	tests := []struct {
		name       string
		profiles   VersionedProfiles[string]
		spxVersion string
		want       string
		wantErr    error
	}{
		{name: "patch above a profile", profiles: withCatchAll, spxVersion: "0.7.5", want: previous},
		{name: "exact profile version", profiles: withCatchAll, spxVersion: "0.8.0", want: latest},
		{name: "patch above the latest profile", profiles: withCatchAll, spxVersion: "0.8.7", want: latest},
		{name: "minor above the latest profile", profiles: withCatchAll, spxVersion: "0.9.3", want: latest},
		{name: "catch-all profile for old AZs", profiles: withCatchAll, spxVersion: "0.3.8", want: catchAll},
		{name: "pre-release of a profile version", profiles: withCatchAll, spxVersion: "0.8.0-rc.1", want: latest},
		{name: "leading v", profiles: withCatchAll, spxVersion: "v0.8.1", want: latest},
		{name: "below every profile", profiles: withoutCatchAll, spxVersion: "0.6.9", wantErr: ErrNoVersionProfile},
		{name: "no profiles", profiles: nil, spxVersion: "0.8.0", wantErr: ErrNoVersionProfile},
		{name: "branch name", profiles: withCatchAll, spxVersion: "main", wantErr: errInvalidSPXVersion},
		{name: "wildcard revision", profiles: withCatchAll, spxVersion: "0.8.*", wantErr: errInvalidSPXVersion},
		{name: "empty version", profiles: withCatchAll, spxVersion: "", wantErr: errInvalidSPXVersion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.profiles.For(tt.spxVersion)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("For(%q) error = %v, want %v", tt.spxVersion, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("For(%q) unexpected error: %v", tt.spxVersion, err)
			}
			if got != tt.want {
				t.Errorf("For(%q) = %q, want %q", tt.spxVersion, got, tt.want)
			}
		})
	}
}
