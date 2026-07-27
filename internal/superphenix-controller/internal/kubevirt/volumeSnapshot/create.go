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

	snapschedulerv1 "github.com/backube/snapscheduler/api/v1"
	v1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	MinSchedule            = 1
	MaxSchedule            = 7
	MinRetentionExpiryTime = 24
	MaxRetentionExpiryTime = 960
)

func (info *CreateSnapshotInfo) CreateSnapshot(ctx context.Context, namespace string) error {
	log := logger.GetLogger(ctx)

	metadata := spxId.Metadata{}
	if err := metadata.ConvertToSpxMetadata(info.Metadata); err != nil {
		log.Err(err).Str("namespace", namespace).Any("info", info).Msg("Failed to parse snapshot informations")
		return err
	}

	if info.Spec.Scheduled {
		return info.createScheduledSnapshot(ctx, namespace, &metadata)
	}
	return info.createSnapshot(ctx, namespace, &metadata)
}

func (info *CreateSnapshotInfo) createSnapshot(ctx context.Context, namespace string, metadata *spxId.Metadata) error {
	log := logger.GetLogger(ctx)

	_, err := config.SnapshotClient.VolumeSnapshots(namespace).Create(ctx, &v1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:   metadata.GetResourceEffectiveID(),
			Labels: metadata.GetLabels(),
		},
		Spec: v1.VolumeSnapshotSpec{
			Source: v1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &info.Spec.Source,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Any("info", info).Msg("Error CreateSnapshot creation")
		return err
	}

	return nil
}

func (info *CreateSnapshotInfo) createScheduledSnapshot(ctx context.Context, namespace string, metadata *spxId.Metadata) error {
	log := logger.GetLogger(ctx)

	// Compute cron schedule
	// Add check for schedule

	if info.Spec.Schedule < MinSchedule || info.Spec.Schedule > MaxSchedule {
		err := fmt.Errorf("schedule value must be between 1 and %d", MaxSchedule)
		log.Error().Int("schedule", info.Spec.Schedule).Msg(err.Error())
		return err
	}

	if info.Spec.Retention.ExpiryTime < MinRetentionExpiryTime || info.Spec.Retention.ExpiryTime > MaxRetentionExpiryTime {
		err := fmt.Errorf("expiryTime value must be between 1 and %d", MaxRetentionExpiryTime)
		log.Error().Int("ExpiryTime", info.Spec.Retention.ExpiryTime).Msg(err.Error())
		return err
	}

	randHours := schedule.RandomHourInRange(config.Global.SnapshotSchedule.MinHour, config.Global.SnapshotSchedule.MaxHour)
	randMinutes := rand.IntN(60)
	cronSchedule := fmt.Sprintf("%d %d */%d * *", randMinutes, randHours, info.Spec.Schedule)

	// Validate retention expires
	expires := fmt.Sprintf("%dh", info.Spec.Retention.ExpiryTime)

	// Build claimSelector matchLabels
	matchLabels, err := utils.ParseLabels(info.Spec.LabelSelector, "")
	if err != nil {
		log.Err(err).Msg("Failed to parse label selector")
		return fmt.Errorf("invalid label selector: %w", err)
	}
	matchLabels[spxId.SpxLabelOrganizationID] = metadata.GetOrgID()
	matchLabels[spxId.SpxLabelProjectID] = metadata.GetProjectID()

	// Build snapshotTemplate labels
	snapshotLabels := map[string]string{
		"superphenix.net/generated":       "true",
		spxId.SpxLabelOrganizationID:      metadata.GetOrgID(),
		spxId.SpxLabelProjectID:           metadata.GetProjectID(),
		spxId.SpxLabelResourceEffectiveID: metadata.GetResourceEffectiveID(),
		spxId.SpxLabelResourceLocalID:     metadata.GetResourceLocalID(),
	}

	// Build SnapshotSchedule object
	snapshotSchedule := &snapschedulerv1.SnapshotSchedule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "snapscheduler.backube/v1",
			Kind:       "SnapshotSchedule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   metadata.GetResourceEffectiveID(),
			Labels: metadata.GetLabels(),
		},
		Spec: snapschedulerv1.SnapshotScheduleSpec{
			Disabled: info.Spec.Paused,
			Schedule: cronSchedule,
			ClaimSelector: metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			Retention: snapschedulerv1.SnapshotRetentionSpec{
				Expires: expires,
			},
			SnapshotTemplate: &snapschedulerv1.SnapshotTemplateSpec{
				Labels: snapshotLabels,
			},
		},
	}

	// Convert to unstructured
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(snapshotSchedule)
	if err != nil {
		log.Err(err).Msg("Failed to convert SnapshotSchedule to unstructured")
		return fmt.Errorf("failed to convert SnapshotSchedule to unstructured: %w", err)
	}

	// Create via dynamic client
	_, err = config.DynamicClientSet.Resource(SnapshotScheduleGVR).Namespace(namespace).Create(
		ctx,
		&unstructured.Unstructured{Object: unstructuredObj},
		metav1.CreateOptions{},
	)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Failed to create SnapshotSchedule")
		return fmt.Errorf("failed to create SnapshotSchedule: %w", err)
	}

	return nil
}
