package baas

import (
	"context"

	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

// CanCreateAllScopedBackup check if there is an existing Scheduled Backup with All scope
// Return false if one exist
func CanCreateAllScopedBackup(ctx context.Context, namespace string) (bool, error) {
	log := logger.GetLogger(ctx)
	schedulesMap, err := listSchedules(ctx, namespace)

	if err != nil {
		log.Error().Err(err).Str("method", "canCreateAllScopedBackup").Msg("list schedules failed")
		return false, err
	}

	for _, schedule := range schedulesMap {
		if schedule.Labels[BackupTypeLabelKey] == BackupTypeAll {
			return false, nil
		}
	}

	return true, nil
}
