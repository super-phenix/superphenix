package kaas

import (
	"fmt"
	"net"
	"net/url"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	"k8s.io/client-go/tools/clientcmd"
)

// ShouldRewriteFQDN reports whether the served kubeconfig for the given cluster
// version should have its server endpoint rewritten to the public FQDN.
// Default true; false only when the version is explicitly configured fqdn:false.
func ShouldRewriteFQDN(versions []config.KubeVersionConfig, version string) bool {
	for _, v := range versions {
		if v.Version == version && v.Fqdn != nil {
			return *v.Fqdn
		}
	}
	return true
}

// RewriteFQDN rewrites every cluster server host in the kubeconfig to
// <clusterID>.kaas.<az>.<domain>, preserving the original scheme and port.
// clusterID is the resource effective ID, which already carries the "spx-" prefix.
func RewriteFQDN(raw []byte, clusterID, az, domain string) ([]byte, error) {
	if clusterID == "" || az == "" || domain == "" {
		return nil, fmt.Errorf("clusterID, az and domain are required to build the kubeconfig FQDN")
	}

	cfg, err := clientcmd.Load(raw)
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	host := fmt.Sprintf("%s.kaas.%s.%s", clusterID, az, domain)
	for name, cluster := range cfg.Clusters {
		u, err := url.Parse(cluster.Server)
		if err != nil {
			return nil, fmt.Errorf("parse server for cluster %q: %w", name, err)
		}
		if port := u.Port(); port != "" {
			u.Host = net.JoinHostPort(host, port)
		} else {
			u.Host = host
		}
		cluster.Server = u.String()
	}

	return clientcmd.Write(*cfg)
}
