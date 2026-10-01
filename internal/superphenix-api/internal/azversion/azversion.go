// Package azversion reads the SPX version of an AZ from the operator Cluster CR
// describing it on the management cluster. Products select their configuration
// profile from that version.
package azversion

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

var clusterGVR = schema.GroupVersionResource{Group: "operator.superphenix.net", Version: "v1alpha1", Resource: "clusters"}

const (
	// cacheTTL bounds how long a listing of the Cluster CRs is reused.
	cacheTTL = 30 * time.Second
	// inClusterDestination is the Argo CD destination of the management cluster.
	inClusterDestination = "in-cluster"
)

// ErrUnknownVersion is returned when the SPX version of an AZ can't be read.
var ErrUnknownVersion = errors.New("cannot determine SPX version of AZ")

// Resolver returns the SPX version an AZ runs.
type Resolver interface {
	Version(ctx context.Context, az config.AZConfig) (string, error)
}

// ClusterResolver reads status.superphenixVersion of the Cluster CRs of one
// namespace. Listings are cached for cacheTTL; failures are not cached.
type ClusterResolver struct {
	client    dynamic.Interface
	namespace string

	mu sync.Mutex
	// versions holds the SPX versions of the Cluster CRs by Argo CD destination.
	versions map[string][]string
	fetched  time.Time
}

var _ Resolver = (*ClusterResolver)(nil)

// NewFromKubeconfig builds a resolver with its own cluster connection, resolved
// from the kubeconfig at path (empty means the default loading rules, then the
// in-cluster config).
func NewFromKubeconfig(path, namespace string) (*ClusterResolver, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = path

	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot obtain K8S rest config: %w", err)
	}

	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain K8S dynamic client: %w", err)
	}
	return &ClusterResolver{client: client, namespace: namespace}, nil
}

// Version returns the SPX version of az, read from the Cluster CR whose Argo CD
// destination is az.Destination. Errors wrap ErrUnknownVersion.
func (r *ClusterResolver) Version(ctx context.Context, az config.AZConfig) (string, error) {
	versions, err := r.list(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrUnknownVersion, err.Error())
	}

	matches := versions[az.Destination]
	switch {
	case len(matches) != 1:
		return "", fmt.Errorf("%w: %d clusters with destination %q in namespace %q", ErrUnknownVersion, len(matches), az.Destination, r.namespace)
	case matches[0] == "":
		return "", fmt.Errorf("%w: cluster with destination %q has no status.superphenixVersion", ErrUnknownVersion, az.Destination)
	}
	return matches[0], nil
}

// list returns the cached versions, or lists the Cluster CRs when they expired.
func (r *ClusterResolver) list(ctx context.Context) (map[string][]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.versions != nil && time.Since(r.fetched) < cacheTTL {
		return r.versions, nil
	}

	list, err := r.client.Resource(clusterGVR).Namespace(r.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("cannot list clusters in namespace %q: %s", r.namespace, err.Error())
	}

	versions := make(map[string][]string, len(list.Items))
	for _, item := range list.Items {
		// The operator's destination for a Cluster CR: in-cluster in Local
		// connection mode, its name otherwise.
		destination := item.GetName()
		if mode, _, _ := unstructured.NestedString(item.Object, "spec", "connection", "mode"); mode == "Local" {
			destination = inClusterDestination
		}
		version, _, _ := unstructured.NestedString(item.Object, "status", "superphenixVersion")
		versions[destination] = append(versions[destination], version)
	}
	r.versions, r.fetched = versions, time.Now()
	return versions, nil
}
