package volumeSnapshot

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteSnapshot(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)

	snapshot, err := config.SnapshotClient.VolumeSnapshots(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return deleteSnapshotSchedule(ctx, namespace, name)
		}
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to get snapshot")
		return err
	}

	if err := utils.CheckProjectLabel(snapshot, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("Volume snapshot access denied")
		return err
	}

	if err := utils.IsEditAllowed(snapshot.GetLabels()); err != nil {
		log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	gracePeriod := int64(0)
	err = config.SnapshotClient.VolumeSnapshots(namespace).Delete(ctx, name, k8smetav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to delete snapshot")
		return err
	}

	return nil
}

func deleteSnapshotSchedule(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)

	schedule, err := config.DynamicClientSet.Resource(SnapshotScheduleGVR).Namespace(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to get snapshot schedule")
		return err
	}

	if err := utils.CheckProjectLabel(schedule, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("Snapshot schedule access denied")
		return err
	}

	if err := utils.IsEditAllowed(schedule.GetLabels()); err != nil {
		log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	gracePeriod := int64(0)
	err = config.DynamicClientSet.Resource(SnapshotScheduleGVR).Namespace(namespace).Delete(ctx, name, k8smetav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to delete snapshot schedule")
		return err
	}

	return nil
}
