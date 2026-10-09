package kaas

import (
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/view"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
)

var (
	testDefaultChart = config.RepoArgoAppConfig{RepoURL: "ghcr.io/super-phenix/charts", Chart: "sfs-kaas", TargetRevision: "0.7.1"}
	testLegacyChart  = config.RepoArgoAppConfig{RepoURL: "ghcr.io/super-phenix/charts", Chart: "sfs-kaas", TargetRevision: "0.3.8"}
)

func testProfile() config.KaasVersionProfile {
	return config.KaasVersionProfile{
		Repo: testDefaultChart,
		KubeVersions: []config.KubeVersionConfig{
			{Version: "v1.36.3"},
			{Version: "v1.34.8", Repo: &testLegacyChart},
		},
	}
}

func TestChartFromConfig(t *testing.T) {

	tests := []struct {
		name        string
		kubeVersion string
		want        config.RepoArgoAppConfig
		wantOk      bool
	}{
		{name: "default repo", kubeVersion: "v1.36.3", want: testDefaultChart, wantOk: true},
		{name: "per version override", kubeVersion: "v1.34.8", want: testLegacyChart, wantOk: true},
		{name: "unsupported version", kubeVersion: "v1.30.0", want: config.RepoArgoAppConfig{}, wantOk: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ChartFromConfig(testProfile(), tt.kubeVersion)
			if ok != tt.wantOk || got != tt.want {
				t.Errorf("ChartFromConfig(%q) = %+v, %v, want %+v, %v", tt.kubeVersion, got, ok, tt.want, tt.wantOk)
			}
		})
	}
}

func TestChartFromApp(t *testing.T) {
	tests := []struct {
		name string
		src  *view.ApplicationSource
		want config.RepoArgoAppConfig
	}{
		{
			name: "helm chart source",
			src:  &view.ApplicationSource{RepoURL: "ghcr.io/super-phenix/charts", Chart: "sfs-kaas", TargetRevision: "0.3.8"},
			want: testLegacyChart,
		},
		{
			name: "path source",
			src:  &view.ApplicationSource{RepoURL: "https://example.org/repo.git", Path: "charts/sfs-kaas", TargetRevision: "main"},
			want: config.RepoArgoAppConfig{RepoURL: "https://example.org/repo.git", Path: "charts/sfs-kaas", TargetRevision: "main"},
		},
		{name: "nil source", src: nil, want: config.RepoArgoAppConfig{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var app view.AppView
			app.Spec.Source = tt.src
			if got := ChartFromApp(app); got != tt.want {
				t.Errorf("ChartFromApp() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestChartForUpdate(t *testing.T) {
	pinned := config.RepoArgoAppConfig{RepoURL: "ghcr.io/super-phenix/charts", Chart: "sfs-kaas", TargetRevision: "0.3.8"}

	tests := []struct {
		name       string
		oldVersion string
		newVersion string
		want       config.RepoArgoAppConfig
		wantOk     bool
	}{
		{name: "same version keeps pinned chart", oldVersion: "v1.36.3", newVersion: "v1.36.3", want: pinned, wantOk: true},
		{name: "same removed version keeps pinned chart", oldVersion: "v1.30.0", newVersion: "v1.30.0", want: pinned, wantOk: true},
		{name: "version change takes configured chart", oldVersion: "v1.34.8", newVersion: "v1.36.3", want: testDefaultChart, wantOk: true},
		{name: "version change takes per version override", oldVersion: "v1.36.3", newVersion: "v1.34.8", want: testLegacyChart, wantOk: true},
		{name: "version change to unsupported version", oldVersion: "v1.36.3", newVersion: "v1.30.0", want: config.RepoArgoAppConfig{}, wantOk: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ChartForUpdate(testProfile(), pinned, tt.oldVersion, tt.newVersion)
			if ok != tt.wantOk || got != tt.want {
				t.Errorf("ChartForUpdate() = %+v, %v, want %+v, %v", got, ok, tt.want, tt.wantOk)
			}
		})
	}
}

func TestSameSource(t *testing.T) {
	tests := []struct {
		name string
		a, b config.RepoArgoAppConfig
		want bool
	}{
		{name: "identical", a: testDefaultChart, b: testDefaultChart, want: true},
		{name: "different revision", a: testDefaultChart, b: testLegacyChart, want: false},
		{name: "different repo", a: testDefaultChart, b: config.RepoArgoAppConfig{RepoURL: "example.org/charts", Chart: "sfs-kaas", TargetRevision: "0.7.1"}, want: false},
		{name: "different chart", a: testDefaultChart, b: config.RepoArgoAppConfig{RepoURL: "ghcr.io/super-phenix/charts", Chart: "other", TargetRevision: "0.7.1"}, want: false},
		{name: "different path", a: config.RepoArgoAppConfig{Path: "a"}, b: config.RepoArgoAppConfig{Path: "b"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SameSource(tt.a, tt.b); got != tt.want {
				t.Errorf("SameSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewChartStatus(t *testing.T) {
	defaultInfo := chartInfo(testDefaultChart)
	legacyInfo := chartInfo(testLegacyChart)

	tests := []struct {
		name        string
		current     config.RepoArgoAppConfig
		kubeVersion string
		want        ChartStatus
	}{
		{
			name: "up to date", current: testDefaultChart, kubeVersion: "v1.36.3",
			want: ChartStatus{Current: defaultInfo, Target: &defaultInfo, Outdated: false},
		},
		{
			name: "pinned below configured chart", current: testLegacyChart, kubeVersion: "v1.36.3",
			want: ChartStatus{Current: legacyInfo, Target: &defaultInfo, Outdated: true},
		},
		{
			name: "per version override matches", current: testLegacyChart, kubeVersion: "v1.34.8",
			want: ChartStatus{Current: legacyInfo, Target: &legacyInfo, Outdated: false},
		},
		{
			name: "kube version removed from config", current: testLegacyChart, kubeVersion: "v1.30.0",
			want: ChartStatus{Current: legacyInfo, Target: nil, Outdated: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewChartStatus(testProfile(), tt.current, tt.kubeVersion)
			if got.Current != tt.want.Current || got.Outdated != tt.want.Outdated {
				t.Errorf("NewChartStatus() = %+v, want %+v", got, tt.want)
			}
			if (got.Target == nil) != (tt.want.Target == nil) || (got.Target != nil && *got.Target != *tt.want.Target) {
				t.Errorf("NewChartStatus().Target = %+v, want %+v", got.Target, tt.want.Target)
			}
		})
	}
}

func TestDeployedOutdated(t *testing.T) {
	profile := testProfile()
	build := config.RepoArgoAppConfig{RepoURL: "ghcr.io/super-phenix/charts", Chart: "sfs-kaas", TargetRevision: "0.8.0+build.1"}
	gitPath := config.RepoArgoAppConfig{RepoURL: "https://github.com/super-phenix/superphenix", Path: "components/dependencies/sfs-kaas", TargetRevision: "main"}
	profile.KubeVersions = append(profile.KubeVersions,
		config.KubeVersionConfig{Version: "v1.37.0", Repo: &build},
		config.KubeVersionConfig{Version: "v1.35.0", Repo: &gitPath},
	)

	tests := []struct {
		name         string
		chartLabel   string
		kubeVersion  string
		wantOutdated bool
		wantKnown    bool
	}{
		{name: "same chart", chartLabel: "sfs-kaas-0.7.1", kubeVersion: "v1.36.3", wantOutdated: false, wantKnown: true},
		{name: "older version", chartLabel: "sfs-kaas-0.3.8", kubeVersion: "v1.36.3", wantOutdated: true, wantKnown: true},
		{name: "per version override", chartLabel: "sfs-kaas-0.3.8", kubeVersion: "v1.34.8", wantOutdated: false, wantKnown: true},
		{name: "other chart same version", chartLabel: "sfs-kaas-edge-0.7.1", kubeVersion: "v1.36.3", wantOutdated: true, wantKnown: true},
		{name: "version with build metadata", chartLabel: "sfs-kaas-0.8.0_build.1", kubeVersion: "v1.37.0", wantOutdated: false, wantKnown: true},
		{name: "kube version not configured", chartLabel: "sfs-kaas-0.3.8", kubeVersion: "v1.30.0", wantOutdated: true, wantKnown: true},
		{name: "path based target", chartLabel: "sfs-kaas-0.0.0", kubeVersion: "v1.35.0", wantOutdated: false, wantKnown: false},
		{name: "empty label", chartLabel: "", kubeVersion: "v1.36.3", wantOutdated: false, wantKnown: false},
		{name: "empty kube version", chartLabel: "sfs-kaas-0.7.1", kubeVersion: "", wantOutdated: false, wantKnown: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outdated, known := DeployedOutdated(profile, tt.chartLabel, tt.kubeVersion)
			if outdated != tt.wantOutdated || known != tt.wantKnown {
				t.Errorf("DeployedOutdated() = (%v, %v), want (%v, %v)", outdated, known, tt.wantOutdated, tt.wantKnown)
			}
		})
	}
}
