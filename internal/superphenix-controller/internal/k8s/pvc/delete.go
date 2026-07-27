package pvc

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeletePVC(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)

	pvcToDelete, err := config.K8sClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to get PVC")
		return err
	}

	if err := utils.CheckProjectLabel(pvcToDelete, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("PVC access denied")
		return err
	}

	if err := utils.IsEditAllowed(pvcToDelete.GetLabels()); err != nil {
		log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	gracePeriod := int64(0)
	err = config.K8sClient.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, k8smetav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	})
	if apierrors.IsNotFound(err) {
		log.Warn().Err(err).Str("namespace", namespace).Str("name", name).Msg("PVC not found")
		return nil
	}
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to delete disk")
		return err
	}

	return nil
}
