package kaas

import (
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	"k8s.io/client-go/tools/clientcmd"
)

func boolPtr(b bool) *bool { return &b }

func TestShouldRewriteFQDN(t *testing.T) {
	versions := []config.KubeVersionConfig{
		{Version: "v1.34.8", Fqdn: boolPtr(false)},
		{Version: "v1.34.1", Fqdn: boolPtr(true)},
		{Version: "v1.33.4"}, // Fqdn nil
	}

	tests := []struct {
		name     string
		versions []config.KubeVersionConfig
		version  string
		want     bool
	}{
		{name: "listed fqdn false", versions: versions, version: "v1.34.8", want: false},
		{name: "listed fqdn true", versions: versions, version: "v1.34.1", want: true},
		{name: "listed fqdn nil defaults true", versions: versions, version: "v1.33.4", want: true},
		{name: "not listed defaults true", versions: versions, version: "v1.35.5", want: true},
		{name: "empty version empty list defaults true", versions: nil, version: "", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldRewriteFQDN(tt.versions, tt.version); got != tt.want {
				t.Errorf("ShouldRewriteFQDN(%v, %q) = %v, want %v", tt.versions, tt.version, got, tt.want)
			}
		})
	}
}

func kubeconfigYAML(servers ...string) string {
	out := "apiVersion: v1\nkind: Config\ncurrent-context: ctx\ncontexts:\n- name: ctx\n  context:\n    cluster: c0\n    user: u1\nusers:\n- name: u1\n  user: {}\nclusters:\n"
	for i, s := range servers {
		out += "- name: c" + string(rune('0'+i)) + "\n  cluster:\n    server: " + s + "\n"
	}
	return out
}

func TestRewriteFQDN(t *testing.T) {
	const (
		// clusterID is the resource effective ID, which already carries the "spx-" prefix.
		clusterID = "spx-eefbc9b5-ee3c-5854-a543-403ea86d6bc5"
		// external is the AZ URL template, "%s" standing for the cluster ID.
		external = "%s.kaas.aq01-test01.superphenix.net"
	)
	wantHost := "spx-eefbc9b5-ee3c-5854-a543-403ea86d6bc5.kaas.aq01-test01.superphenix.net"

	tests := []struct {
		name        string
		raw         string
		clusterID   string
		external    string
		wantErr     bool
		wantServers map[string]string // cluster name -> expected server
	}{
		{
			name:        "single cluster preserves port",
			raw:         kubeconfigYAML("https://10.96.0.1:7443"),
			clusterID:   clusterID,
			external:    external,
			wantServers: map[string]string{"c0": "https://" + wantHost + ":7443"},
		},
		{
			name:        "scheme preserved",
			raw:         kubeconfigYAML("http://10.96.0.1:7443"),
			clusterID:   clusterID,
			external:    external,
			wantServers: map[string]string{"c0": "http://" + wantHost + ":7443"},
		},
		{
			name:        "no port in source",
			raw:         kubeconfigYAML("https://10.96.0.1"),
			clusterID:   clusterID,
			external:    external,
			wantServers: map[string]string{"c0": "https://" + wantHost},
		},
		{
			name:      "multiple clusters each keep their port",
			raw:       kubeconfigYAML("https://10.96.0.1:7443", "https://10.96.0.2:6443"),
			clusterID: clusterID,
			external:  external,
			wantServers: map[string]string{
				"c0": "https://" + wantHost + ":7443",
				"c1": "https://" + wantHost + ":6443",
			},
		},
		{
			name:        "ipv6 source host with port",
			raw:         kubeconfigYAML("https://[fd00::1]:7443"),
			clusterID:   clusterID,
			external:    external,
			wantServers: map[string]string{"c0": "https://" + wantHost + ":7443"},
		},
		{
			name:      "malformed kubeconfig",
			raw:       "this is not a kubeconfig: {{{",
			clusterID: clusterID,
			external:  external,
			wantErr:   true,
		},
		{
			name:      "unparseable server url",
			raw:       kubeconfigYAML("https://%zz"),
			clusterID: clusterID,
			external:  external,
			wantErr:   true,
		},
		{
			name:      "missing external url",
			raw:       kubeconfigYAML("https://10.96.0.1:7443"),
			clusterID: clusterID,
			external:  "",
			wantErr:   true,
		},
		{
			name:      "external url without placeholder",
			raw:       kubeconfigYAML("https://10.96.0.1:7443"),
			clusterID: clusterID,
			external:  "kaas.aq01-test01.superphenix.net",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RewriteFQDN([]byte(tt.raw), tt.clusterID, tt.external)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			cfg, err := clientcmd.Load(got)
			if err != nil {
				t.Fatalf("output is not a valid kubeconfig: %v", err)
			}
			for name, wantServer := range tt.wantServers {
				cluster, ok := cfg.Clusters[name]
				if !ok {
					t.Fatalf("cluster %q missing from output", name)
				}
				if cluster.Server != wantServer {
					t.Errorf("cluster %q server = %q, want %q", name, cluster.Server, wantServer)
				}
			}
			// non-server fields survive
			if cfg.CurrentContext != "ctx" {
				t.Errorf("current-context = %q, want ctx", cfg.CurrentContext)
			}
			if _, ok := cfg.AuthInfos["u1"]; !ok {
				t.Errorf("user u1 missing from output")
			}
		})
	}
}
