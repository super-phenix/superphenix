package natGateway

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/vpc"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteNatGateway(ctx context.Context, namespace, vpcEID, name string) error {
	log := logger.GetLogger(ctx)

	natGwToDelete, err := k8s.KubeOvnClient.KubeovnV1().VpcNatGateways().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		log.Err(err).Str("name", name).Msg("Failed to get NatGw")
		return err
	}

	if err := utils.CheckProjectLabel(natGwToDelete, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("Nat Gateway access denied")
		return err
	}

	if err := utils.IsEditAllowed(natGwToDelete.GetLabels()); err != nil {
		log.Error().Err(err).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	if err := vpc.RemoveNatGateway(ctx, vpcEID, name, natGwToDelete.Spec.LanIP); err != nil {
		log.Err(err).Str("vpcEID", vpcEID).Str("name", name).Msg("Failed to remove NatGw from VPC")
		return err
	}

	err = k8s.KubeOvnClient.KubeovnV1().VpcNatGateways().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		log.Err(err).Str("name", name).Msg("Failed to remove NatGw")
		return err
	}

	return nil
}
