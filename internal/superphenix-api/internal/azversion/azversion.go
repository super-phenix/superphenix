// Package azversion reads the SPX version of an AZ from the operator Cluster CR
// describing it on the management cluster.
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

// ClusterGVR identifies the operator Cluster CRD, one object per AZ.
var ClusterGVR = schema.GroupVersionResource{Group: "operator.superphenix.net", Version: "v1alpha1", Resource: "clusters"}

// inClusterDestination is the Argo CD destination of the AZ hosted by the
// management cluster; it doesn't name its Cluster CR.
const inClusterDestination = "in-cluster"

// cacheTTL bounds how long a listing of the Cluster CRs is reused.
const cacheTTL = 30 * time.Second

// ErrUnknownVersion is returned when the SPX version of an AZ can't be read.
var ErrUnknownVersion = errors.New("cannot determine SPX version of AZ")

// Resolver returns the SPX version an AZ runs.
type Resolver interface {
	Version(ctx context.Context, az config.AZConfig) (string, error)
}

// ClusterResolver reads status.superphenixVersion of the Cluster CRs listed in
// one namespace. Listings are cached for cacheTTL; failures are not cached.
type ClusterResolver struct {
	client    dynamic.Interface
	namespace string
	ttl       time.Duration
	now       func() time.Time

	mu       sync.Mutex
	clusters *snapshot
}

var _ Resolver = (*ClusterResolver)(nil)

type cluster struct {
	name    string
	az      string
	version string
}

type snapshot struct {
	byName  map[string]cluster
	byAZ    map[string][]cluster
	fetched time.Time
}

// New builds a resolver over the Cluster CRs of namespace.
func New(client dynamic.Interface, namespace string) *ClusterResolver {
	return &ClusterResolver{client: client, namespace: namespace, ttl: cacheTTL, now: time.Now}
}

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
	return New(client, namespace), nil
}

// Version returns the SPX version of az. Its Cluster CR is az.ClusterName,
// else az.Destination, else the one whose spec.availabilityZone is az.Code when
// the destination is in-cluster. Errors wrap ErrUnknownVersion.
func (r *ClusterResolver) Version(ctx context.Context, az config.AZConfig) (string, error) {
	snap, err := r.snapshot(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrUnknownVersion, err.Error())
	}

	c, err := snap.find(az, r.namespace)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrUnknownVersion, err.Error())
	}
	if c.version == "" {
		return "", fmt.Errorf("%w: cluster %q has no status.superphenixVersion", ErrUnknownVersion, c.name)
	}
	return c.version, nil
}

// snapshot returns the cached listing, or lists the Cluster CRs when it expired.
func (r *ClusterResolver) snapshot(ctx context.Context) (*snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.clusters != nil && r.now().Sub(r.clusters.fetched) < r.ttl {
		return r.clusters, nil
	}

	list, err := r.client.Resource(ClusterGVR).Namespace(r.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("cannot list clusters in namespace %q: %s", r.namespace, err.Error())
	}

	snap := &snapshot{
		byName:  make(map[string]cluster, len(list.Items)),
		byAZ:    make(map[string][]cluster, len(list.Items)),
		fetched: r.now(),
	}
	for _, item := range list.Items {
		c := toCluster(item)
		snap.byName[c.name] = c
		if c.az != "" {
			snap.byAZ[c.az] = append(snap.byAZ[c.az], c)
		}
	}
	r.clusters = snap
	return snap, nil
}

func (s *snapshot) find(az config.AZConfig, namespace string) (cluster, error) {
	name := az.ClusterName
	if name == "" && az.Destination != inClusterDestination {
		name = az.Destination
	}

	if name != "" {
		c, ok := s.byName[name]
		if !ok {
			return cluster{}, fmt.Errorf("cluster %q not found in namespace %q", name, namespace)
		}
		return c, nil
	}

	matches := s.byAZ[az.Code]
	switch len(matches) {
	case 0:
		return cluster{}, fmt.Errorf("no cluster with availabilityZone %q in namespace %q", az.Code, namespace)
	case 1:
		return matches[0], nil
	default:
		return cluster{}, fmt.Errorf("%d clusters with availabilityZone %q in namespace %q, set azs.%s.clusterName", len(matches), az.Code, namespace, az.Code)
	}
}

func toCluster(item unstructured.Unstructured) cluster {
	az, _, _ := unstructured.NestedString(item.Object, "spec", "availabilityZone")
	version, _, _ := unstructured.NestedString(item.Object, "status", "superphenixVersion")
	return cluster{name: item.GetName(), az: az, version: version}
}
