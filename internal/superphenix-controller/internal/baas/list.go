package baas

import (
	"context"
	"slices"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListBaaS(ctx context.Context, namespace string) ([]view.BaasObject, error) {
	log := logger.GetLogger(ctx)

	// List schedule
	schedulesMap, err := listSchedules(ctx, namespace)
	if err != nil {
		log.Err(err).Msg("failed to list schedules")
		return nil, err
	}
	// List backup
	backupsMap, err := listBackups(ctx, namespace, "")
	if err != nil {
		log.Err(err).Msg("failed to list backups")
		return nil, err
	}

	var res []view.BaasObject
	var usedSchedule []string
	for schedEID, backups := range backupsMap {
		schedule := view.Schedule{}
		orphan := false
		standalone := false
		if schedEID == BackupStandaloneKey {
			standalone = true
		} else {
			if sched, ok := schedulesMap[schedEID]; ok {
				schedule = sched
				schedule.Backups = backups

				usedSchedule = append(usedSchedule, schedule.Name)

				res = append(res, view.BaasObject{
					ObjectMeta: schedule.ObjectMeta,
					Schedule:   &schedule,
				})
				continue
			} else {
				orphan = true
			}
		}

		for _, backup := range backups {
			res = append(res, view.BaasObject{
				ObjectMeta: backup.ObjectMeta,
				Backup:     &backup,
				Standalone: standalone,
				Orphan:     orphan,
			})
		}
	}

	for eid, schedule := range schedulesMap {
		if !slices.Contains(usedSchedule, eid) {
			res = append(res, view.BaasObject{
				ObjectMeta: schedule.ObjectMeta,
				Schedule:   &schedule,
			})
		}

	}

	return res, nil
}

func listSchedules(ctx context.Context, namespace string) (map[string]view.Schedule, error) {
	log := logger.GetLogger(ctx)

	list, err := informers.WatcherSet[informers.Schedule].ByIndex("namespace", BackupNamespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("error listing schedules")
		return nil, err
	}

	scheduleList := map[string]view.Schedule{}
	for _, item := range list {
		schedule := item.(*unstructured.Unstructured)

		if schedule.GetLabels()[spxId.SpxLabelProjectID] == namespace &&
			schedule.GetLabels()[BackupAzLocationLabelKey] == config.Global.AzName {
			scheduleView := unstructuredToSchedule(*schedule)
			scheduleList[schedule.GetName()] = scheduleView
		}
	}

	return scheduleList, nil
}

func listBackups(ctx context.Context, namespace string, scheduleNameFilter string) (map[string][]view.Backup, error) {
	log := logger.GetLogger(ctx)

	list, err := informers.WatcherSet[informers.Backup].ByIndex("namespace", BackupNamespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("error listing backups")
		return nil, err
	}

	backupList := map[string][]view.Backup{}
	for _, item := range list {
		backup := item.(*unstructured.Unstructured)

		if backup.GetLabels()[spxId.SpxLabelProjectID] == namespace &&
			backup.GetLabels()[BackupAzLocationLabelKey] == config.Global.AzName {

			if scheduleNameFilter == "" || backup.GetLabels()[BackupScheduleNameLabelKey] == scheduleNameFilter {
				backupView := unstructuredToBackup(*backup)

				scheduleName := backup.GetLabels()[BackupScheduleNameLabelKey]
				if scheduleName == "" {
					scheduleName = BackupStandaloneKey
				}

				if backupList[scheduleName] == nil {
					backupList[scheduleName] = []view.Backup{backupView}
				} else {
					backupList[scheduleName] = append(backupList[scheduleName], backupView)
				}
			}

		}
	}

	return backupList, nil
}
