package subnet

import (
	"context"

	natGateway "github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/nat_gateway"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubevirt/nad"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteSubnet(ctx context.Context, namespace string, name string) error {
	log := logger.GetLogger(ctx)

	subnetToDelete, err := k8s.KubeOvnClient.KubeovnV1().Subnets().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to get Subnet")
		return err
	}

	if err := utils.CheckProjectLabel(subnetToDelete, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("Subnet access denied")
		return err
	}

	if err := utils.IsEditAllowed(subnetToDelete.GetLabels()); err != nil {
		log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	if err := nad.DeleteNAD(ctx, namespace, name); err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to delete NAD")
		return err
	}

	natGw, err := natGateway.GetNatGw(ctx, namespace, name)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to get NatGw")
		return err
	}
	// If we found a natGW
	if natGw != nil {
		if err := natGateway.DeleteNatGateway(ctx, namespace, natGw.Spec.Vpc, natGw.Name); err != nil {
			log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to delete NatGw")
			return err
		}
	}

	if err := k8s.KubeOvnClient.KubeovnV1().Subnets().Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to delete Subnet")
		return err
	}
	return nil
}
