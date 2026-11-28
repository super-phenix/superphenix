package dbaas

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetDBaaS(ctx context.Context, namespace, effectiveId string) (view.DBaaS, error) {
	cluster, err := getCluster(ctx, namespace, effectiveId)
	if err != nil {
		return view.DBaaS{}, err
	}

	return clusterToResource(ctx, cluster), nil
}

// getCluster returns the Cluster when it belongs to the project.
func getCluster(ctx context.Context, namespace, effectiveId string) (*unstructured.Unstructured, error) {
	cluster, err := config.DynamicClientSet.Resource(ClusterGVR).Namespace(namespace).Get(ctx, effectiveId, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if err := utils.CheckProjectLabel(cluster, namespace); err != nil {
		return nil, err
	}

	return cluster, nil
}
