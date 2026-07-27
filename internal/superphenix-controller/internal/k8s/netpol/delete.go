package netpol

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteNetPol(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)

	netPol, err := k8s.K8sClient.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("method", "DeleteNetPol").Str("namespace", namespace).Str("name", name).Msg("Error getting network policy")
		return err
	}

	if err := utils.CheckProjectLabel(netPol, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("NetworkPolicy access denied")
		return err
	}

	if err := utils.IsEditAllowed(netPol.GetLabels()); err != nil {
		log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	err = k8s.K8sClient.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		log.Err(err).Str("method", "DeleteNetPol").Str("namespace", namespace).Str("name", name).Msg("Error deleting network policy")
		return err
	}
	return nil
}
