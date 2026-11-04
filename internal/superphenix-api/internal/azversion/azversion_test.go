package azversion

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const testNamespace = "superphenix-system"

func newCluster(namespace, name, az, mode, version string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "operator.superphenix.net/v1alpha1",
		"kind":       "Cluster",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
		"spec": map[string]any{
			"availabilityZone": az,
			"connection":       map[string]any{"mode": mode},
		},
	}}
	if version != "" {
		obj.Object["status"] = map[string]any{"superphenixVersion": version}
	}
	return obj
}

func newFakeClient(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{clusterGVR: "ClusterList"}, objs...)
}

func TestClusterResolverVersion(t *testing.T) {
	clusters := []runtime.Object{
		newCluster(testNamespace, "spx-aq01-test01-virt01", "aq01-test01", "Remote", "0.8.2"),
		newCluster(testNamespace, "spx-aq01-test01-storage01", "aq01-test01", "Remote", "0.7.5"),
		newCluster(testNamespace, "management", "local", "Local", "0.9.0-rc.1"),
		newCluster(testNamespace, "no-status", "aq01-test03", "Remote", ""),
		newCluster("other-namespace", "elsewhere", "aq01-test04", "Remote", "0.8.0"),
	}

	tests := []struct {
		name     string
		clusters []runtime.Object
		az       config.AZConfig
		want     string
		wantErr  string
	}{
		{
			name: "destination names a remote cluster",
			az:   config.AZConfig{Code: "aq01-test01", Destination: "spx-aq01-test01-virt01"},
			want: "0.8.2",
		},
		{
			name: "clusters sharing an availability zone are told apart by destination",
			az:   config.AZConfig{Code: "aq01-test01", Destination: "spx-aq01-test01-storage01"},
			want: "0.7.5",
		},
		{
			name: "in-cluster destination matches the local cluster",
			az:   config.AZConfig{Code: "local", Destination: "in-cluster"},
			want: "0.9.0-rc.1",
		},
		{
			name:    "local cluster is not reachable by its name",
			az:      config.AZConfig{Code: "local", Destination: "management"},
			wantErr: `0 clusters with destination "management"`,
		},
		{
			name:    "unknown destination",
			az:      config.AZConfig{Code: "aq01-test01", Destination: "missing"},
			wantErr: `0 clusters with destination "missing" in namespace "superphenix-system"`,
		},
		{
			name:    "cluster without status version",
			az:      config.AZConfig{Code: "aq01-test03", Destination: "no-status"},
			wantErr: `cluster with destination "no-status" has no status.superphenixVersion`,
		},
		{
			name:    "cluster of another namespace is ignored",
			az:      config.AZConfig{Code: "aq01-test04", Destination: "elsewhere"},
			wantErr: `0 clusters with destination "elsewhere"`,
		},
		{
			name: "several local clusters are ambiguous",
			clusters: []runtime.Object{
				newCluster(testNamespace, "local-a", "a", "Local", "0.8.0"),
				newCluster(testNamespace, "local-b", "b", "Local", "0.8.0"),
			},
			az:      config.AZConfig{Code: "a", Destination: "in-cluster"},
			wantErr: `2 clusters with destination "in-cluster"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := clusters
			if tt.clusters != nil {
				objs = tt.clusters
			}
			r := &ClusterResolver{client: newFakeClient(objs...), namespace: testNamespace}
			got, err := r.Version(context.Background(), tt.az)
			if tt.wantErr != "" {
				if !errors.Is(err, ErrUnknownVersion) || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Version() error = %v, want ErrUnknownVersion containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Version() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Version() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClusterResolverCache(t *testing.T) {
	az := config.AZConfig{Code: "aq01-test01", Destination: "spx-aq01-test01-virt01"}

	tests := []struct {
		name      string
		failFirst bool
		elapsed   time.Duration
		wantLists int
		wantErrs  int
	}{
		{name: "listing reused within ttl", elapsed: 10 * time.Second, wantLists: 1},
		{name: "listing refreshed after ttl", elapsed: cacheTTL, wantLists: 2},
		{name: "failed listing is not cached", failFirst: true, elapsed: 0, wantLists: 2, wantErrs: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(newCluster(testNamespace, "spx-aq01-test01-virt01", "aq01-test01", "Remote", "0.8.2"))
			lists := 0
			client.PrependReactor("list", "clusters", func(k8stesting.Action) (bool, runtime.Object, error) {
				lists++
				if tt.failFirst && lists == 1 {
					return true, nil, errors.New("connection refused")
				}
				return false, nil, nil
			})

			r := &ClusterResolver{client: client, namespace: testNamespace}

			errs := 0
			for i := range 2 {
				if i == 1 {
					r.fetched = r.fetched.Add(-tt.elapsed)
				}
				if _, err := r.Version(context.Background(), az); err != nil {
					errs++
				}
			}
			if lists != tt.wantLists || errs != tt.wantErrs {
				t.Errorf("lists = %d, errors = %d, want %d, %d", lists, errs, tt.wantLists, tt.wantErrs)
			}
		})
	}
}

func TestNewFromKubeconfig(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "kubeconfig")
	kubeconfig := `apiVersion: v1
kind: Config
current-context: test
clusters:
  - name: test
    cluster:
      server: https://127.0.0.1:6443
users:
  - name: test
    user:
      token: test-token
contexts:
  - name: test
    context:
      cluster: test
      user: test
`
	if err := os.WriteFile(valid, []byte(kubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "explicit kubeconfig", path: valid},
		{name: "missing explicit kubeconfig", path: filepath.Join(dir, "missing"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewFromKubeconfig(tt.path, testNamespace)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewFromKubeconfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && r.namespace != testNamespace {
				t.Errorf("namespace = %q, want %q", r.namespace, testNamespace)
			}
		})
	}
}
