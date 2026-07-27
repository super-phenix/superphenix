package volumeSnapshot

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
	"github.com/super-phenix/superphenix/pkg/utils/schedule"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (info *UpdateSnapshotInfo) UpdateSnapshot(ctx context.Context, namespace, effectiveId string) error {
	log := logger.GetLogger(ctx)

	// Fetch existing SnapshotSchedule
	existing, err := config.DynamicClientSet.Resource(SnapshotScheduleGVR).Namespace(namespace).Get(ctx, effectiveId, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error().Str("effectiveId", effectiveId).Msg("SnapshotSchedule not found, only SnapshotSchedules can be updated")
			return err
		}
		log.Err(err).Str("namespace", namespace).Str("effectiveId", effectiveId).Msg("Failed to get SnapshotSchedule")
		return err
	}

	if err := utils.CheckProjectLabel(existing, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("effectiveId", effectiveId).Msg("Snapshot schedule access denied")
		return err
	}

	// Check edit allowed
	if err := utils.IsEditAllowed(existing.GetLabels()); err != nil {
		log.Error().Err(err).Str("namespace", namespace).Str("effectiveId", effectiveId).Msg("Edit not allowed on this resource")
		return err
	}

	// Validate schedule
	if info.Spec.Schedule < MinSchedule || info.Spec.Schedule > MaxSchedule {
		err := fmt.Errorf("schedule value must be between %d and %d", MinSchedule, MaxSchedule)
		log.Error().Int("schedule", info.Spec.Schedule).Msg(err.Error())
		return err
	}

	// Validate retention
	if info.Spec.Retention.ExpiryTime < MinRetentionExpiryTime || info.Spec.Retention.ExpiryTime > MaxRetentionExpiryTime {
		err := fmt.Errorf("expiryTime value must be between %d and %d", MinRetentionExpiryTime, MaxRetentionExpiryTime)
		log.Error().Int("ExpiryTime", info.Spec.Retention.ExpiryTime).Msg(err.Error())
		return err
	}

	// Recompute cron schedule
	randHours := schedule.RandomHourInRange(config.Global.SnapshotSchedule.MinHour, config.Global.SnapshotSchedule.MaxHour)
	randMinutes := rand.IntN(60)
	cronSchedule := fmt.Sprintf("%d %d */%d * *", randMinutes, randHours, info.Spec.Schedule)

	// Compute retention expires
	expires := fmt.Sprintf("%dh", info.Spec.Retention.ExpiryTime)

	// Build claimSelector matchLabels
	matchLabels, err := utils.ParseLabels(info.Spec.LabelSelector, "")
	if err != nil {
		log.Err(err).Msg("Failed to parse label selector")
		return fmt.Errorf("invalid label selector: %w", err)
	}
	// Add org/project labels from existing object
	existingLabels := existing.GetLabels()
	matchLabels[spxId.SpxLabelOrganizationID] = existingLabels[spxId.SpxLabelOrganizationID]
	matchLabels[spxId.SpxLabelProjectID] = existingLabels[spxId.SpxLabelProjectID]

	// Update spec fields in the unstructured object
	spec, ok := existing.Object["spec"].(map[string]interface{})
	if !ok {
		spec = make(map[string]interface{})
	}

	spec["disabled"] = info.Spec.Paused
	spec["schedule"] = cronSchedule
	spec["claimSelector"] = map[string]interface{}{
		"matchLabels": matchLabels,
	}
	spec["retention"] = map[string]interface{}{
		"expires": expires,
	}

	existing.Object["spec"] = spec

	// Update via dynamic client
	_, err = config.DynamicClientSet.Resource(SnapshotScheduleGVR).Namespace(namespace).Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("effectiveId", effectiveId).Msg("Failed to update SnapshotSchedule")
		return fmt.Errorf("failed to update SnapshotSchedule: %w", err)
	}

	return nil
}
