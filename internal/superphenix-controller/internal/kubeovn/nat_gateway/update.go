package natGateway

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UpdateNatGwInfo struct {
	LanIP string
}

func (info UpdateNatGwInfo) UpdateNatGateway(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)
	natGwToUpdate, err := k8s.KubeOvnClient.KubeovnV1().VpcNatGateways().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Any("info", info).Msg("Failed to get Nat Gateway")
		return err
	}

	if err := utils.CheckProjectLabel(natGwToUpdate, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("Nat Gateway access denied")
		return err
	}

	if err := utils.IsEditAllowed(natGwToUpdate.GetLabels()); err != nil {
		log.Error().Err(err).Any("info", info).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	natGwToUpdate.Spec.LanIP = info.LanIP
	if _, err := k8s.KubeOvnClient.KubeovnV1().VpcNatGateways().Update(ctx, natGwToUpdate, metav1.UpdateOptions{}); err != nil {
		log.Err(err).Msg("Failed to update Nat Gateway")
		return err
	}

	return nil
}
