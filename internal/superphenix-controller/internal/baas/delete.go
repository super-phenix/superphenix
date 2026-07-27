package baas

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func DeleteBaaS(ctx context.Context, namespace, eid string) error {
	log := logger.GetLogger(ctx)

	// Pre-flight: confirm the Backup/Schedule lives in the caller's project by
	// fetching it from the velero namespace and checking its project label.
	// The Backup CR comes first; if missing, fall back to the Schedule CR.
	if err := checkBaaSOwnership(ctx, namespace, eid); err != nil {
		log.Warn().Str("namespace", namespace).Str("eid", eid).Msg("BaaS access denied")
		return err
	}

	deleteRequest := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "velero.io/v1",
			"kind":       "DeleteBackupRequest",
			"metadata": map[string]interface{}{
				"name":      eid,
				"namespace": BackupNamespace,
			},
			"spec": map[string]interface{}{
				"backupName": eid,
			},
		},
	}

	deleteBackupRequestResources := k8s.DynamicClientSet.Resource(DeleteBackupRequestGVR).Namespace(BackupNamespace)
	_, err := deleteBackupRequestResources.Create(ctx, deleteRequest, metav1.CreateOptions{})
	if err != nil {
		log.Error().Err(err).Str("eid", eid).Msg("Failed to create DeleteBackupRequest")
		return err
	}

	log.Info().Str("eid", eid).Msg("DeleteBackupRequest created successfully")
	return nil
}

func checkBaaSOwnership(ctx context.Context, namespace, eid string) error {
	backupResources := k8s.DynamicClientSet.Resource(BackupGVR).Namespace(BackupNamespace)
	unstructBackup, err := backupResources.Get(ctx, eid, metav1.GetOptions{})
	if err == nil {
		return utils.CheckProjectLabel(unstructBackup, namespace)
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	scheduleResources := k8s.DynamicClientSet.Resource(ScheduleGVR).Namespace(BackupNamespace)
	unstructSched, err := scheduleResources.Get(ctx, eid, metav1.GetOptions{})
	if err != nil {
		return err
	}
	return utils.CheckProjectLabel(unstructSched, namespace)
}
