package kaas

import (
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/view"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
)

// ChartInfo identifies the chart an Application renders.
type ChartInfo struct {
	RepoURL        string `json:"repoURL"`
	Chart          string `json:"chart,omitempty"`
	Path           string `json:"path,omitempty"`
	TargetRevision string `json:"targetRevision"`
}

// ChartStatus compares the chart a cluster renders with the configured one.
// Target is nil when the cluster's kube version is not configured.
type ChartStatus struct {
	Current  ChartInfo  `json:"current"`
	Target   *ChartInfo `json:"target,omitempty"`
	Outdated bool       `json:"outdated"`
}

// ChartFromConfig returns the chart profile configures for kubeVersion and
// whether the version is supported.
func ChartFromConfig(profile config.KaasVersionProfile, kubeVersion string) (config.RepoArgoAppConfig, bool) {
	return config.ResolveKubeVersionRepo(profile.KubeVersions, profile.Repo, kubeVersion)
}

// ChartFromApp returns the chart the Application renders.
func ChartFromApp(app view.AppView) config.RepoArgoAppConfig {
	src := app.Spec.Source
	if src == nil {
		return config.RepoArgoAppConfig{}
	}
	return config.RepoArgoAppConfig{
		RepoURL:        src.RepoURL,
		TargetRevision: src.TargetRevision,
		Chart:          src.Chart,
		Path:           src.Path,
	}
}

// ChartForUpdate returns the chart an updated cluster renders. It keeps
// current unless the kube version changes, in which case it takes the chart
// profile configures for newVersion. ok is false when newVersion is not configured.
func ChartForUpdate(profile config.KaasVersionProfile, current config.RepoArgoAppConfig, oldVersion, newVersion string) (config.RepoArgoAppConfig, bool) {
	if oldVersion == newVersion {
		return current, true
	}
	return ChartFromConfig(profile, newVersion)
}

// SameSource reports whether a and b point to the same chart revision.
func SameSource(a, b config.RepoArgoAppConfig) bool {
	return a.RepoURL == b.RepoURL && a.Chart == b.Chart && a.Path == b.Path && a.TargetRevision == b.TargetRevision
}

// NewChartStatus builds the status of a cluster rendering current with the
// given kube version, against the chart profile configures.
func NewChartStatus(profile config.KaasVersionProfile, current config.RepoArgoAppConfig, kubeVersion string) ChartStatus {
	status := ChartStatus{Current: chartInfo(current)}
	target, ok := ChartFromConfig(profile, kubeVersion)
	if !ok {
		status.Outdated = true
		return status
	}
	info := chartInfo(target)
	status.Target = &info
	status.Outdated = !SameSource(current, target)
	return status
}

// DeployedOutdated reports whether chartLabel, the helm.sh/chart label of a
// deployed cluster, differs from the chart profile configures for kubeVersion.
// known is false when an input is empty or the configured chart is path based.
func DeployedOutdated(profile config.KaasVersionProfile, chartLabel, kubeVersion string) (outdated, known bool) {
	if chartLabel == "" || kubeVersion == "" {
		return false, false
	}
	target, ok := ChartFromConfig(profile, kubeVersion)
	if !ok {
		return true, true
	}
	if target.Path != "" {
		return false, false
	}
	return chartLabel != helmChartLabel(target.Chart, target.TargetRevision), true
}

// helmChartLabel renders the helm.sh/chart label like the sfs-kaas.chart helper.
func helmChartLabel(name, version string) string {
	label := strings.ReplaceAll(name+"-"+version, "+", "_")
	if len(label) > 63 {
		label = label[:63]
	}
	return strings.TrimSuffix(label, "-")
}

func chartInfo(c config.RepoArgoAppConfig) ChartInfo {
	return ChartInfo{RepoURL: c.RepoURL, Chart: c.Chart, Path: c.Path, TargetRevision: c.TargetRevision}
}
