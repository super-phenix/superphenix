package volumeSnapshot

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetSnapshot(ctx context.Context, namespace, name string) (view.Snapshot, error) {
	log := logger.GetLogger(ctx)

	snapshot, err := config.SnapshotClient.VolumeSnapshots(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err == nil {
		if err := utils.CheckProjectLabel(snapshot, namespace); err != nil {
			log.Warn().Str("namespace", namespace).Str("name", name).Msg("Volume snapshot access denied")
			return view.Snapshot{}, err
		}
		snapshotView := view.SnapshotToView(*snapshot)
		return view.SnapshotViewToResource(snapshotView), nil
	}

	if !errors.IsNotFound(err) {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error getting snapshot")
		return view.Snapshot{}, err
	}

	// Fallback: try fetching as SnapshotSchedule
	scheduleResource, scheduleErr := getSnapshotSchedule(ctx, namespace, name)
	if scheduleErr != nil {
		if errors.IsNotFound(scheduleErr) {
			log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Resource not found as VolumeSnapshot or SnapshotSchedule")
			return view.Snapshot{}, err
		}
		log.Err(scheduleErr).Str("namespace", namespace).Str("name", name).Msg("Error getting snapshot schedule")
		return view.Snapshot{}, scheduleErr
	}

	return scheduleResource, nil
}

func getSnapshotSchedule(ctx context.Context, namespace, name string) (view.Snapshot, error) {
	log := logger.GetLogger(ctx)

	unstructuredSchedule, err := config.DynamicClientSet.Resource(SnapshotScheduleGVR).Namespace(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error getting snapshot schedule")
		return view.Snapshot{}, err
	}

	if err := utils.CheckProjectLabel(unstructuredSchedule, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("Snapshot schedule access denied")
		return view.Snapshot{}, err
	}

	scheduleView := view.UnstructuredToSnapshotScheduleView(unstructuredSchedule)

	// Fetch child VolumeSnapshots via OwnerReference
	snapshotList, err := config.SnapshotClient.VolumeSnapshots(namespace).List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error listing snapshots for schedule children")
		return view.Snapshot{}, err
	}

	scheduleName := unstructuredSchedule.GetName()
	var children []view.SnapshotView
	for _, snap := range snapshotList.Items {
		for _, ownerRef := range snap.GetOwnerReferences() {
			if ownerRef.Name == scheduleName {
				children = append(children, view.SnapshotToView(snap))
				break
			}
		}
	}
	scheduleView.Children = children
	scheduleView.ChildrenCount = len(children)

	return view.SnapshotScheduleViewToResource(scheduleView), nil
}
