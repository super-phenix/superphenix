package config

import (
	"errors"

	"github.com/spf13/viper"
)

// KubeVersionConfig is a supported Kubernetes version with an optional chart
// repo override and an optional FQDN-rewrite flag. When Repo is set it fully
// replaces the default KaaS repo. Fqdn nil means true (the default for >=1.35
// and newer versions); legacy <1.35 versions set fqdn:false.
type KubeVersionConfig struct {
	Version string             `yaml:"version"`
	Repo    *RepoArgoAppConfig `yaml:"repo,omitempty"`
	Fqdn    *bool              `yaml:"fqdn"`
}

// AzDomainConfig holds the URLs used in KaaS cluster configuration for one AZ.
// Internal is the host the in-cluster kube-API proxy resolves to reach control
// planes, including across AZs during disaster recovery. External is a URL
// template for the control plane FQDN, where "%s" is replaced by the cluster ID.
type AzDomainConfig struct {
	Internal string `yaml:"internal"`
	External string `yaml:"external"`
}

// KaasAzDomain returns the azDomains entry for az. ok is false for string
// entries and missing keys.
func (c *Config) KaasAzDomain(az string) (AzDomainConfig, bool) {
	entry, ok := c.ProductsConfig.ArgoApp.Kubernetes.AzDomains[az].(map[string]any)
	if !ok {
		return AzDomainConfig{}, false
	}
	internal, _ := entry["internal"].(string)
	external, _ := entry["external"].(string)
	return AzDomainConfig{Internal: internal, External: external}, true
}

// KaasVersionProfile is the KaaS configuration of the AZs selected by its key
// in KubernetesArgoAppConfig.Versions.
type KaasVersionProfile struct {
	Repo         RepoArgoAppConfig   `yaml:"repo"`
	KubeVersions []KubeVersionConfig `yaml:"kubeVersions"`
}

func (p KaasVersionProfile) validate() []string {
	var errs []string
	if p.Repo.RepoURL == "" || p.Repo.TargetRevision == "" {
		errs = append(errs, ".repo must set repoURL and targetRevision")
	}
	if len(p.KubeVersions) == 0 {
		errs = append(errs, ".kubeVersions must not be empty")
	}
	return errs
}

// KubernetesArgoAppConfig is the KaaS product configuration.
type KubernetesArgoAppConfig struct {
	// Versions holds the KaaS profiles, keyed by minimum AZ SPX version.
	Versions VersionedProfiles[KaasVersionProfile] `yaml:"versions"`
	// sfs-kaas `azDomains` value. Entry shapes:
	//   <az code>: {internal, external}  sfs-kaas >= 0.7.0
	//   <region>: <domain>               sfs-kaas < 0.7.0
	AzDomains map[string]any `yaml:"azDomains"`
}

// Chart returns the chart p configures for kubeVersion and whether the version
// is supported. A version's own Repo fully replaces p.Repo.
func (p KaasVersionProfile) Chart(kubeVersion string) (RepoArgoAppConfig, bool) {
	for _, v := range p.KubeVersions {
		if v.Version == kubeVersion {
			if v.Repo != nil {
				return *v.Repo, true
			}
			return p.Repo, true
		}
	}
	return RepoArgoAppConfig{}, false
}

// checkRemovedKeys rejects the old kubernetes.repo and kubeVersions keys, to
// ensure they were moved to kubernetes.versions instead of silently ignored.
// The repo sub-keys catch env variables.
func checkRemovedKeys(v *viper.Viper) error {
	for _, key := range []string{"repo", "repo|repoURL", "repo|chart", "repo|targetRevision", "kubeVersions"} {
		if v.IsSet("productsConfig|argoApp|kubernetes|" + key) {
			return errors.New("invalid configuration: productsConfig.argoApp.kubernetes.repo and .kubeVersions were moved under " +
				`productsConfig.argoApp.kubernetes.versions.<minimum AZ SPX version>, e.g. versions."0.0.0".repo`)
		}
	}
	return nil
}
