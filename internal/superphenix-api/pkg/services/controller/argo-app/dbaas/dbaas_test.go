package dbaas

import (
	"context"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
)

const testProjectID = "6f1c2c1e-3b8a-4b8e-9d4a-2f1e5c7b9a10"

func setPostgresVersions(t *testing.T, versions ...string) {
	t.Helper()
	d := &config.Global.ProductsConfig.ArgoApp.DBaaS
	prev := d.PostgresVersions
	t.Cleanup(func() { d.PostgresVersions = prev })
	d.PostgresVersions = make([]config.PostgresVersionConfig, 0, len(versions))
	for _, v := range versions {
		d.PostgresVersions = append(d.PostgresVersions, config.PostgresVersionConfig{Version: v})
	}
}

func validSpec() DBaaSSpec {
	return DBaaSSpec{
		Version:      "17.6",
		Instances:    2,
		Cpu:          2,
		Memory:       4,
		Storage:      20,
		StorageClass: "standard",
		SubnetEId:    "spx-subnet",
		VIP:          "198.18.1.10",
	}
}

func TestCreateAppValues(t *testing.T) {
	setPostgresVersions(t, "17.6", "17.7", "18.1")
	subnet := Subnet{LocalID: "subnet-local", CIDR: "10.10.0.0/24"}
	old := validSpec()

	tests := []struct {
		name    string
		spec    func(DBaaSSpec) DBaaSSpec
		oldSpec *DBaaSSpec
		wantErr string
		want    []string
	}{
		{
			name: "create",
			spec: func(s DBaaSSpec) DBaaSSpec { return s },
			want: []string{
				"databases:\n  db-1:\n",
				`version: "17.6"`,
				"instances: 2",
				`cpu: "2"`,
				"memory: 4Gi",
				"storage: 20Gi",
				"storageClassName: standard",
				"subnet: subnet-local",
				"subnetCIDR: 10.10.0.0/24",
				"vip: 198.18.1.10",
				"location: az1",
			},
		},
		{
			name: "external access",
			spec: func(s DBaaSSpec) DBaaSSpec { s.ExternalAccess = true; return s },
			want: []string{"externalAccess: true"},
		},
		{name: "too many instances", spec: func(s DBaaSSpec) DBaaSSpec { s.Instances = 6; return s }, wantErr: "instances"},
		{name: "no instance", spec: func(s DBaaSSpec) DBaaSSpec { s.Instances = 0; return s }, wantErr: "instances"},
		{name: "bad cpu", spec: func(s DBaaSSpec) DBaaSSpec { s.Cpu = 3; return s }, wantErr: "cpu"},
		{name: "bad memory", spec: func(s DBaaSSpec) DBaaSSpec { s.Memory = 3; return s }, wantErr: "memory"},
		{name: "no storage", spec: func(s DBaaSSpec) DBaaSSpec { s.Storage = 0; return s }, wantErr: "storage"},
		{name: "vip out of range", spec: func(s DBaaSSpec) DBaaSSpec { s.VIP = "10.0.0.1"; return s }, wantErr: "vip"},
		{name: "unsupported version", spec: func(s DBaaSSpec) DBaaSSpec { s.Version = "16.1"; return s }, wantErr: "not supported"},
		{name: "invalid version", spec: func(s DBaaSSpec) DBaaSSpec { s.Version = "latest"; return s }, wantErr: "invalid version"},
		{
			name:    "update minor version and resources",
			spec:    func(s DBaaSSpec) DBaaSSpec { s.Version = "17.7"; s.Instances = 3; s.Cpu = 4; s.Storage = 40; return s },
			oldSpec: &old,
			want:    []string{`version: "17.7"`, "instances: 3", `cpu: "4"`, "storage: 40Gi"},
		},
		{
			name:    "update keeps omitted immutable fields",
			spec:    func(s DBaaSSpec) DBaaSSpec { s.SubnetEId, s.VIP, s.StorageClass = "", "", ""; return s },
			oldSpec: &old,
			want:    []string{"vip: 198.18.1.10", "storageClassName: standard"},
		},
		{name: "major change", spec: func(s DBaaSSpec) DBaaSSpec { s.Version = "18.1"; return s }, oldSpec: &old, wantErr: "major"},
		{name: "storage shrink", spec: func(s DBaaSSpec) DBaaSSpec { s.Storage = 10; return s }, oldSpec: &old, wantErr: "reduced"},
		{name: "storage class change", spec: func(s DBaaSSpec) DBaaSSpec { s.StorageClass = "fast"; return s }, oldSpec: &old, wantErr: "storage class"},
		{name: "subnet change", spec: func(s DBaaSSpec) DBaaSSpec { s.SubnetEId = "spx-other"; return s }, oldSpec: &old, wantErr: "subnet"},
		{name: "vip change", spec: func(s DBaaSSpec) DBaaSSpec { s.VIP = "198.18.1.11"; return s }, oldSpec: &old, wantErr: "vip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateAppValues(context.Background(), "db-1", "az1", tt.spec(validSpec()), subnet, tt.oldSpec)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("CreateAppValues() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateAppValues() unexpected error: %v", err)
			}
			for _, s := range tt.want {
				if !strings.Contains(got, s) {
					t.Errorf("values do not contain %q:\n%s", s, got)
				}
			}
		})
	}
}

func TestCreateArgoApp(t *testing.T) {
	setPostgresVersions(t, "17.6")
	az := config.AZConfig{Code: "az1", Destination: "cluster-az1"}
	classes := []ClassMapping{{Shortname: "standard", Fullname: "ceph-rbd"}}

	tests := []struct {
		name           string
		class          string
		externalAccess bool
		externalSubnet string
		wantErr        bool
		wantParams     []string
	}{
		{name: "available class", class: "standard", wantParams: []string{"--set storageClassMapping.standard=ceph-rbd"}},
		{name: "unknown class", class: "fast", wantErr: true},
		{
			name: "external access", class: "standard", externalAccess: true, externalSubnet: "external",
			wantParams: []string{"--set externalSubnet=external"},
		},
		{name: "external access without external subnet", class: "standard", externalAccess: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := validSpec()
			spec.StorageClass = tt.class
			spec.ExternalAccess = tt.externalAccess
			cfg := DBaaSConfig{StorageClasses: classes, ExternalSubnet: tt.externalSubnet}
			m := testMetadata(t)
			info, err := CreateArgoApp(context.Background(), "db-1", az, spec, Subnet{LocalID: "s", CIDR: "10.0.0.0/24"}, m, cfg, nil, config.RepoArgoAppConfig{Chart: "sfs-dbaas"})
			if (err != nil) != tt.wantErr {
				t.Fatalf("CreateArgoApp() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if info.General.AppName != "dbaas-"+m.GetResourceEffectiveID() {
				t.Errorf("AppName = %s", info.General.AppName)
			}
			params := pluginEnv(info.Spec.Source.Plugin.Env, "HELM_PARAMS")
			for _, p := range tt.wantParams {
				if !strings.Contains(params, p) {
					t.Errorf("HELM_PARAMS = %q, want it to contain %q", params, p)
				}
			}
		})
	}
}

func TestChartForUpdate(t *testing.T) {
	override := config.RepoArgoAppConfig{Chart: "sfs-dbaas", TargetRevision: "0.2.0"}
	d := &config.Global.ProductsConfig.ArgoApp.DBaaS
	prevRepo, prevVersions := d.Repo, d.PostgresVersions
	t.Cleanup(func() { d.Repo, d.PostgresVersions = prevRepo, prevVersions })
	d.Repo = config.RepoArgoAppConfig{Chart: "sfs-dbaas", TargetRevision: "0.1.0"}
	d.PostgresVersions = []config.PostgresVersionConfig{{Version: "17.6"}, {Version: "17.7", Repo: &override}}
	current := config.RepoArgoAppConfig{Chart: "sfs-dbaas", TargetRevision: "0.0.9"}

	tests := []struct {
		name       string
		oldVersion string
		newVersion string
		want       config.RepoArgoAppConfig
		wantOK     bool
	}{
		{name: "same version keeps chart", oldVersion: "17.5", newVersion: "17.5", want: current, wantOK: true},
		{name: "default repo", oldVersion: "17.5", newVersion: "17.6", want: d.Repo, wantOK: true},
		{name: "version override", oldVersion: "17.6", newVersion: "17.7", want: override, wantOK: true},
		{name: "unsupported", oldVersion: "17.6", newVersion: "17.8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ChartForUpdate(current, tt.oldVersion, tt.newVersion)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("ChartForUpdate() = %+v, %v, want %+v, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
