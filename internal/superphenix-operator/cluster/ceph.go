package cluster

import (
	"context"
	"encoding/json"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

var cephClusterGVR = schema.GroupVersionResource{
	Group:    "ceph.rook.io",
	Version:  "v1",
	Resource: "cephclusters",
}

func (r *Reconciler) reconcileCephClusters(ctx context.Context, config *rest.Config) (map[string]apiextensionsv1.JSON, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	cephClusters, err := dynamicClient.Resource(cephClusterGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		// If the CRD is not registered, we just return an empty map or nil.
		// This can happen if Rook is not installed on the target cluster.
		return nil, nil
	}

	result := make(map[string]apiextensionsv1.JSON)
	for _, cephCluster := range cephClusters.Items {
		cephStatus, found, err := unstructured.NestedMap(cephCluster.Object, "status", "ceph")
		if err != nil || !found {
			continue
		}

		fsid, found, err := unstructured.NestedString(cephStatus, "fsid")
		if err != nil || !found || fsid == "" {
			continue
		}

		// Convert cephStatus to JSON bytes
		bytes, err := json.Marshal(cephStatus)
		if err != nil {
			continue
		}

		result[fsid] = apiextensionsv1.JSON{Raw: bytes}
	}

	return result, nil
}
