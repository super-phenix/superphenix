package volumeSnapshot

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListSnapshots(ctx context.Context, namespace string) []view.Snapshot {
	log := logger.GetLogger(ctx)
	if namespace == "" {
		log.Error().Msg("No namespace provided")
		return make([]view.Snapshot, 0)
	}

	// List all VolumeSnapshots
	list, err := informers.WatcherSet[informers.VolumeSnapshot].ByIndex("namespace", namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error listing snapshots")
		return make([]view.Snapshot, 0)
	}

	allSnapshots := make([]view.SnapshotView, 0, len(list))
	for _, item := range list {
		snapshot := item.(*unstructured.Unstructured)
		allSnapshots = append(allSnapshots, view.UnstructuredSnapshotToView(snapshot))
	}

	// List all SnapshotSchedules
	scheduleList, err := config.DynamicClientSet.Resource(SnapshotScheduleGVR).Namespace(namespace).List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error listing snapshot schedules")
		// Return standalone snapshots only
		result := make([]view.Snapshot, 0, len(allSnapshots))
		for _, s := range allSnapshots {
			result = append(result, view.SnapshotViewToResource(s))
		}
		return result
	}

	// Reconcile snapshots and schedules
	scheduleViews := make([]view.SnapshotScheduleView, 0, len(scheduleList.Items))
	for _, scheduleItem := range scheduleList.Items {
		scheduleViews = append(scheduleViews, view.UnstructuredToSnapshotScheduleView(&scheduleItem))
	}

	return reconcileSnapshotsAndSchedules(allSnapshots, scheduleViews)
}

// reconcileSnapshotsAndSchedules groups child snapshots under their parent schedules
// and returns standalone snapshots (not owned by any schedule) alongside schedule resources.
func reconcileSnapshotsAndSchedules(allSnapshots []view.SnapshotView, scheduleViews []view.SnapshotScheduleView) []view.Snapshot {
	matchedSnapshotNames := make(map[string]struct{})
	scheduleResources := make([]view.Snapshot, 0, len(scheduleViews))

	for _, scheduleView := range scheduleViews {
		childrenCount := 0
		for _, snap := range allSnapshots {
			for _, ownerRef := range snap.OwnerReferences {
				if ownerRef.Name == scheduleView.Name {
					childrenCount++
					matchedSnapshotNames[snap.Name] = struct{}{}
					break
				}
			}
		}
		scheduleView.ChildrenCount = childrenCount
		scheduleResources = append(scheduleResources, view.SnapshotScheduleViewToResource(scheduleView))
	}

	result := make([]view.Snapshot, 0, len(allSnapshots))
	for _, snap := range allSnapshots {
		if _, matched := matchedSnapshotNames[snap.Name]; !matched {
			result = append(result, view.SnapshotViewToResource(snap))
		}
	}

	result = append(result, scheduleResources...)

	return result
}
