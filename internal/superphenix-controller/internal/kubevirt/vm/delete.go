package vm

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteVM(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)

	vmToDelete, err := config.VirtClient.VirtualMachine(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error getting VM")
		return err
	}

	if err := utils.CheckProjectLabel(vmToDelete, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("VM access denied")
		return err
	}

	if err := utils.IsEditAllowed(vmToDelete.GetLabels()); err != nil {
		log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	gracePeriod := int64(0)
	err = config.VirtClient.VirtualMachine(namespace).Delete(ctx, name, k8smetav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error deleting VM")
		return err
	}

	return nil
}
