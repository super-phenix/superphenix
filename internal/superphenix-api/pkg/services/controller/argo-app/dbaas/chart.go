package dbaas

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/view"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
)

// ChartFromConfig returns the configured chart for version and whether the
// version is supported.
func ChartFromConfig(version string) (config.RepoArgoAppConfig, bool) {
	d := config.Global.ProductsConfig.ArgoApp.DBaaS
	return config.ResolvePostgresVersionRepo(d.PostgresVersions, d.Repo, version)
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

// ChartForUpdate returns the chart an updated database renders. It keeps
// current unless the version changes, in which case it takes the chart
// configured for newVersion. ok is false when newVersion is not configured.
func ChartForUpdate(current config.RepoArgoAppConfig, oldVersion, newVersion string) (config.RepoArgoAppConfig, bool) {
	if oldVersion == newVersion {
		return current, true
	}
	return ChartFromConfig(newVersion)
}
