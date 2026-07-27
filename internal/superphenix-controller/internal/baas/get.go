package baas

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetBaaS(ctx context.Context, namespace, eid string) (view.BaasObject, error) {
	log := logger.GetLogger(ctx)
	if namespace == "" {
		log.Error().Msg("No namespace provided")
		return view.BaasObject{}, fmt.Errorf("no namespace provided")
	}

	scheduleResources := k8s.DynamicClientSet.Resource(ScheduleGVR).Namespace(BackupNamespace)
	unstructSched, err := scheduleResources.Get(ctx, eid, metav1.GetOptions{})
	// If no schedule found
	if err != nil && apierrors.IsNotFound(err) {
		// Check backupsMap
		backupResources := k8s.DynamicClientSet.Resource(BackupGVR).Namespace(BackupNamespace)
		unstructBackup, err := backupResources.Get(ctx, eid, metav1.GetOptions{})
		if err != nil {
			log.Error().Err(err).Str("eid", eid).Msg("No resource matching eid found")
			return view.BaasObject{}, err
		}

		if err := utils.CheckProjectLabel(unstructBackup, namespace); err != nil {
			log.Warn().Str("namespace", namespace).Str("eid", eid).Msg("Backup access denied")
			return view.BaasObject{}, err
		}

		backup := unstructuredToBackup(*unstructBackup)

		orphan := false
		standalone := false
		scheduleName := unstructBackup.GetLabels()[BackupScheduleNameLabelKey]
		if scheduleName == "" {
			standalone = true
		} else {
			// Check if the referenced schedule actually exists
			_, schedErr := scheduleResources.Get(ctx, scheduleName, metav1.GetOptions{})
			if schedErr != nil && apierrors.IsNotFound(schedErr) {
				orphan = true
			} else if schedErr != nil {
				log.Warn().Err(schedErr).Str("scheduleName", scheduleName).Msg("Error checking schedule existence, assuming orphan")
				orphan = true
			}
		}

		return view.BaasObject{
			ObjectMeta: backup.ObjectMeta,
			Backup:     &backup,
			Standalone: standalone,
			Orphan:     orphan,
		}, nil

	} else if err != nil && !apierrors.IsNotFound(err) {
		log.Error().Err(err).Str("eid", eid).Msg("Error getting Schedule resource")
		return view.BaasObject{}, err
	}

	if err := utils.CheckProjectLabel(unstructSched, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("eid", eid).Msg("Backup schedule access denied")
		return view.BaasObject{}, err
	}

	schedule := unstructuredToSchedule(*unstructSched)

	backupsMap, err := listBackups(ctx, namespace, eid)
	if err != nil {
		log.Error().Err(err).Str("eid", eid).Msg("Error listing backups")
		return view.BaasObject{}, err
	}

	schedule.Backups = backupsMap[eid]

	return view.BaasObject{
		ObjectMeta: schedule.ObjectMeta,
		Schedule:   &schedule,
	}, nil
}
