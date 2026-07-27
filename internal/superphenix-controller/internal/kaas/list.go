package kaas

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ListCluster(ctx context.Context, namespace string) ([]view.Cluster, error) {
	log := logger.GetLogger(ctx)

	clusterResources := k8s.DynamicClientSet.Resource(clustersGVR).Namespace(namespace)
	unstructuredList, err := clusterResources.List(ctx, metav1.ListOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error getting cluster list")
		return []view.Cluster{}, err
	}

	clusters := make([]view.Cluster, 0)
	for _, item := range unstructuredList.Items {
		if err := utils.CheckProjectLabel(&item, namespace); err != nil {
			continue
		}
		cluster := unstructuredToCluster(item)
		deployments, err := listMachineDeployments(ctx, namespace, cluster.Name)
		if err != nil {
			log.Err(err).Str("namespace", namespace).Str("clusterName", cluster.Name).Msg("Error getting machine deployment list")
		} else {
			clusters = append(clusters, view.Cluster{
				Cluster:            cluster,
				MachineDeployments: deployments,
			})
		}

	}

	return clusters, nil
}

func listMachineDeployments(ctx context.Context, namespace, clusterName string) ([]view.MachineDeployment, error) {
	log := logger.GetLogger(ctx)
	mdR := k8s.DynamicClientSet.Resource(machineDeploymentsGVR).Namespace(namespace)

	unstructuredList, err := mdR.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", ClusterLabelKey, clusterName),
	})

	if apierrors.IsNotFound(err) {
		return []view.MachineDeployment{}, nil
	}
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("clusterName", clusterName).Msg("Error getting machine deployment for cluster")
		return []view.MachineDeployment{}, err
	}

	mdList := make([]view.MachineDeployment, 0)
	for _, item := range unstructuredList.Items {
		md := unstructuredToMachineDeployment(item)
		mdList = append(mdList, view.MachineDeployment{
			MachineDeployment: md,
		})
	}

	return mdList, nil
}
