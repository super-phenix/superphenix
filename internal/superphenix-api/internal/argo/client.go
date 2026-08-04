// Package argo is the infrastructure adapter over the Kubernetes and Argo CD
// APIs. It used to live behind an HTTP hop in the standalone argo-controller
// binary; the routes are gone and the API now calls these methods directly.
package argo

import (
	"fmt"
	"time"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/typed/application/v1alpha1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// GCOptions configures the garbage collection sweep and the on-demand marking.
type GCOptions struct {
	Enabled      bool
	Interval     time.Duration
	Timeout      time.Duration
	Delay        time.Duration
	LabelMarkKey string
	// Debug turns the sweep into a dry run: candidates are logged, not deleted.
	Debug bool
}

// Options carries everything the client needs that isn't a Kubernetes connection.
type Options struct {
	// Kubeconfig is an explicit path; empty means the default loading rules
	// (KUBECONFIG, ~/.kube/config) then in-cluster config.
	Kubeconfig          string
	AppProjectNamespace string
	GC                  GCOptions
}

// Client wraps the Argo CD and Kubernetes clientsets. A nil *Client is a
// supported state for callers: it means no cluster was reachable at startup.
type Client struct {
	apps v1alpha1.ArgoprojV1alpha1Interface
	k8s  kubernetes.Interface
	opts Options
}

// NewClient builds a client over already-constructed clientsets. Used by tests.
func NewClient(apps v1alpha1.ArgoprojV1alpha1Interface, k8s kubernetes.Interface, opts Options) *Client {
	return &Client{apps: apps, k8s: k8s, opts: opts}
}

// NewClientFromKubeconfig resolves a cluster connection from the kubeconfig
// loading rules (or in-cluster config) and builds both clientsets. It returns an
// error rather than exiting: superphenix-api boots without a cluster.
func NewClientFromKubeconfig(opts Options) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = opts.Kubeconfig

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot obtain K8S rest config: %w", err)
	}

	k8sClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain K8S clientset: %w", err)
	}

	argoClient, err := v1alpha1.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain Argo client: %w", err)
	}

	return NewClient(argoClient, k8sClient, opts), nil
}

// Apps exposes the Argo CD typed client to the garbage collector.
func (c *Client) Apps() v1alpha1.ArgoprojV1alpha1Interface { return c.apps }

// Options returns the client configuration.
func (c *Client) Options() Options { return c.opts }

// Namespace returns the cluster namespace backing a project, e.g. spx-<projectId>.
func (c *Client) Namespace(projectId string) string { return spxId.ToSPXID(projectId) }
