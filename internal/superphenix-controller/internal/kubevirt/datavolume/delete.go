package datavolume

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteDisk(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)

	disk, err := config.VirtClient.CdiClient().CdiV1beta1().DataVolumes(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to get data volume")
		return err
	}

	if err := utils.CheckProjectLabel(disk, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("Disk access denied")
		return err
	}

	if err := utils.IsEditAllowed(disk.GetLabels()); err != nil {
		log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	gracePeriod := int64(0)
	err = config.VirtClient.CdiClient().CdiV1beta1().DataVolumes(namespace).Delete(ctx, name, k8smetav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	})

	if apierrors.IsNotFound(err) {
		log.Warn().Err(err).Str("namespace", namespace).Str("name", name).Msg("Data volume not found")
		return nil
	}
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to delete disk")
		return err
	}

	return nil
}
