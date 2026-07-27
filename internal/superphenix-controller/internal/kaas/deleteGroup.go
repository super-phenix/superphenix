package kaas

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteGroup(ctx context.Context, namespace, effectiveId string, groupList []string) error {
	log := logger.GetLogger(ctx)

	if groupList == nil || len(groupList) == 0 {
		log.Info().Msg("No group list")
		return nil
	}

	clusterResources := k8s.DynamicClientSet.Resource(clustersGVR).Namespace(namespace)
	cluster, err := clusterResources.Get(ctx, effectiveId, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("effectiveId", effectiveId).Msg("Error getting cluster resource")
		return err
	}

	if err := utils.CheckProjectLabel(cluster, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("effectiveId", effectiveId).Msg("Cluster access denied")
		return err
	}

	localId := cluster.GetLabels()[spxId.SpxLabelResourceLocalID]
	if localId == "" {
		log.Error().Str("namespace", namespace).Str("effectiveId", effectiveId).Msg("Cluster localId not found")
		return fmt.Errorf("cluster localId not found")
	}

	mdResources := k8s.DynamicClientSet.Resource(machineDeploymentsGVR).Namespace(namespace)
	for _, group := range groupList {
		resourceName := fmt.Sprintf("%s-%s", localId, group)
		labelSelector := fmt.Sprintf("%s=%s", spxId.SpxLabelResourceName, resourceName)

		err := mdResources.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			log.Err(err).Str("namespace", namespace).Str("resourceName", resourceName).Msg("Error deleting MachineDeployments")
			return err
		}
	}

	return nil
}
