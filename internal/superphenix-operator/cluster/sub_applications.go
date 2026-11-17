package cluster

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
)

const (
	// operationPhaseRunning is the ArgoCD OperationPhase value indicating an in-flight sync.
	operationPhaseRunning = "Running"
	// operationPhaseTerminating is the ArgoCD OperationPhase value used to request termination
	// of an in-flight operation. Writing this to status.operationState.phase is the same signal
	// the argocd CLI sends via `argocd app terminate-op`.
	operationPhaseTerminating = "Terminating"
)

// runPeriodicSync evaluates whether a sub-application sync should be triggered now and triggers
// it. The decision honours:
//   - SyncPeriod: a sync triggered less than SyncPeriod ago short-circuits the run.
//   - In-progress detection: if any Application (root or sub) is already in the Running phase,
//     the run is skipped to avoid queuing duplicate operations.
//   - SyncTimeout: if the in-flight sync has been running longer than SyncTimeout it is assumed
//     to be stuck. ArgoCD is asked to terminate it and a fresh sync is started in the same
//     reconcile, bypassing the per-application "skip if Running" guard.
//
// Returns true if a fresh sync was triggered so the caller can persist LastSync.
func (r *Reconciler) runPeriodicSync(ctx context.Context, cluster *operatorv1alpha1.Cluster) bool {
	log := logf.FromContext(ctx)

	if cluster.Status.LastSync != nil && time.Since(cluster.Status.LastSync.Time) < r.SyncPeriod {
		log.Info("Skipping periodic sync, last sync was recent",
			"lastSync", cluster.Status.LastSync.Time, "syncPeriod", r.SyncPeriod)
		return false
	}

	inProgress, oldestStart, err := r.syncInProgress(ctx, cluster)
	if err != nil {
		log.Error(err, "Failed to check sync state, skipping periodic sync")
		return false
	}

	force := false
	if inProgress {
		if time.Since(oldestStart) < r.SyncTimeout {
			log.Info("Sync already in progress, skipping",
				"startedAt", oldestStart, "syncTimeout", r.SyncTimeout)
			return false
		}
		log.Info("Sync exceeded timeout, aborting and restarting",
			"startedAt", oldestStart, "syncTimeout", r.SyncTimeout)
		if err := r.abortInProgressSyncs(ctx, cluster); err != nil {
			log.Error(err, "Failed to abort stale syncs, will retry on next reconcile")
			return false
		}
		// After abort the cached Application objects may still show phase=Running for a short
		// window before ArgoCD updates them. Force the triggers so the restart is not blocked.
		force = true
	}

	r.syncSubApplications(ctx, cluster, force)
	return true
}

// syncSubApplications triggers an ArgoCD sync on the root Application and on every child
// Application that belongs to the cluster.
//
// In the app of apps pattern, the root Application deploys a Helm chart that in turn contains
// ArgoCD Application manifests. ArgoCD applies those manifests, creating child Applications that
// each manage a specific component of the Superphenix stack. This function ensures the entire
// Application tree is periodically synced so that drift is corrected without relying solely on
// ArgoCD's own internal polling interval.
//
// The mechanism is to set the operation.sync field directly on each Application object, which
// is the programmatic equivalent of running `argocd app sync`. Because operation is a top-level
// field (not under spec), patching it does not change the object's generation, so the
// GenerationChangedPredicate on the Application watch is not triggered and no reconcile loop occurs.
// Failures are non-fatal; individual errors are logged and the function continues with remaining Applications.
func (r *Reconciler) syncSubApplications(ctx context.Context, cluster *operatorv1alpha1.Cluster, force bool) {
	log := logf.FromContext(ctx)

	// Also sync the root Application itself so it re-evaluates the chart and propagates
	// any changes down to the sub-applications on the same cycle.
	rootApp, err := r.fetchApplication(ctx, cluster.Name)
	if err != nil {
		log.Error(err, "Failed to fetch root Application for sync")
	} else if err := r.triggerApplicationSync(ctx, rootApp, force); err != nil {
		log.Error(err, "Failed to sync root Application")
	}

	subApps, err := r.listSubApplications(ctx, cluster)
	if err != nil {
		log.Error(err, "Failed to list sub-applications, skipping periodic sync")
		return
	}

	if len(subApps) == 0 {
		log.Info("No sub-applications found for cluster", "cluster", cluster.Name)
		return
	}

	synced := 0
	for i := range subApps {
		if err := r.triggerApplicationSync(ctx, &subApps[i], force); err != nil {
			log.Error(err, "Failed to sync sub-application", "name", subApps[i].GetName())
			continue
		}
		synced++
	}

	log.Info("Periodic sync triggered", "cluster", cluster.Name, "appsSynced", synced, "appsTotal", len(subApps))
}

// listAllApplications returns the root Application appended to the list of sub-applications for
// the cluster. A missing root Application is non-fatal so callers can still operate on whatever
// sub-applications already exist.
func (r *Reconciler) listAllApplications(ctx context.Context, cluster *operatorv1alpha1.Cluster) ([]unstructured.Unstructured, error) {
	apps, err := r.listSubApplications(ctx, cluster)
	if err != nil {
		return nil, err
	}
	if rootApp, err := r.fetchApplication(ctx, cluster.Name); err == nil {
		apps = append(apps, *rootApp)
	}
	return apps, nil
}

// syncInProgress reports whether any Application owned by the cluster currently has a Running
// operation, and the oldest startedAt timestamp across those operations so the caller can
// compare it against SyncTimeout. An unparseable startedAt is treated as "just started" so a
// missing timestamp never causes a spurious abort.
func (r *Reconciler) syncInProgress(ctx context.Context, cluster *operatorv1alpha1.Cluster) (bool, time.Time, error) {
	apps, err := r.listAllApplications(ctx, cluster)
	if err != nil {
		return false, time.Time{}, err
	}

	inProgress := false
	var oldest time.Time
	for i := range apps {
		phase, _, _ := unstructured.NestedString(apps[i].Object, "status", "operationState", "phase")
		if phase != operationPhaseRunning {
			continue
		}
		inProgress = true

		startedAtStr, _, _ := unstructured.NestedString(apps[i].Object, "status", "operationState", "startedAt")
		startedAt, parseErr := time.Parse(time.RFC3339, startedAtStr)
		if parseErr != nil {
			startedAt = time.Now()
		}
		if oldest.IsZero() || startedAt.Before(oldest) {
			oldest = startedAt
		}
	}
	return inProgress, oldest, nil
}

// abortInProgressSyncs requests ArgoCD to terminate any Running operation on the cluster's
// Applications and clears the .operation field so a fresh sync can be installed in the same
// reconcile. The Terminating phase is written to .status.operationState.phase, matching what
// `argocd app terminate-op` does. The Application CRD has no status subresource, so status is
// patched on the main resource (Status().Patch would return NotFound). Per-application errors
// are logged but do not stop processing of the remaining Applications.
func (r *Reconciler) abortInProgressSyncs(ctx context.Context, cluster *operatorv1alpha1.Cluster) error {
	log := logf.FromContext(ctx)
	apps, err := r.listAllApplications(ctx, cluster)
	if err != nil {
		return err
	}

	for i := range apps {
		phase, _, _ := unstructured.NestedString(apps[i].Object, "status", "operationState", "phase")
		if phase != operationPhaseRunning {
			continue
		}

		// The optimistic lock makes the patch fail with a conflict if Argo CD updated the
		// Application since it was listed (e.g. the operation just finished), instead of
		// overwriting a newer phase with Terminating.
		statusPatch := client.MergeFromWithOptions(apps[i].DeepCopy(), client.MergeFromWithOptimisticLock{})
		if err := unstructured.SetNestedField(apps[i].Object, operationPhaseTerminating, "status", "operationState", "phase"); err != nil {
			log.Error(err, "Failed to build terminate status patch", "name", apps[i].GetName())
			continue
		}
		if err := r.Patch(ctx, &apps[i], statusPatch); err != nil {
			log.Error(err, "Failed to terminate sync via status", "name", apps[i].GetName())
			continue
		}

		// Clear .operation so the subsequent triggerApplicationSync installs a new operation
		// rather than no-oping against the stuck one. JSON merge patch removes fields set to null.
		clearPatch := client.RawPatch(types.MergePatchType, []byte(`{"operation":null}`))
		if err := r.Patch(ctx, &apps[i], clearPatch); err != nil {
			log.Error(err, "Failed to clear operation field", "name", apps[i].GetName())
			continue
		}

		log.Info("Aborted stale sync operation", "name", apps[i].GetName())
	}
	return nil
}

// fetchApplication retrieves a single ArgoCD Application by name from the operator namespace.
func (r *Reconciler) fetchApplication(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Application",
	})

	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: r.OperatorNamespace}, app); err != nil {
		return nil, fmt.Errorf("failed to get Application %s: %w", name, err)
	}

	return app, nil
}

// listSubApplications returns all ArgoCD Applications labelled with the cluster's name.
// Sub-applications are created by ArgoCD when it applies the root Application's Helm chart.
// The chart names them as "{cluster.name}-{component}" and attaches the ClusterLabel so they
// can be listed unambiguously without colliding with sub-applications from other clusters.
func (r *Reconciler) listSubApplications(ctx context.Context, cluster *operatorv1alpha1.Cluster) ([]unstructured.Unstructured, error) {
	appList := &unstructured.UnstructuredList{}
	appList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "ApplicationList",
	})

	if err := r.List(ctx, appList,
		client.InNamespace(r.OperatorNamespace),
		client.MatchingLabels{version.ClusterLabel: cluster.Name},
	); err != nil {
		return nil, fmt.Errorf("failed to list sub-applications for cluster %s: %w", cluster.Name, err)
	}

	return appList.Items, nil
}

// triggerApplicationSync triggers an immediate sync on the given ArgoCD Application by setting
// the operation.sync field, which is the programmatic equivalent of `argocd app sync`.
// ArgoCD detects the field, executes the sync, then clears it.
// A sync is skipped if an operation is already in progress to avoid queue buildup, unless force
// is true: callers that have just aborted a stuck sync use force=true so the cached Running
// phase does not block the restart.
func (r *Reconciler) triggerApplicationSync(ctx context.Context, app *unstructured.Unstructured, force bool) error {
	log := logf.FromContext(ctx)

	if !force {
		phase, _, _ := unstructured.NestedString(app.Object, "status", "operationState", "phase")
		if phase == operationPhaseRunning {
			log.Info("Application already syncing, skipping trigger", "name", app.GetName())
			return nil
		}
	}

	patch := client.MergeFrom(app.DeepCopy())

	if err := unstructured.SetNestedMap(app.Object, map[string]interface{}{
		"sync": map[string]interface{}{},
	}, "operation"); err != nil {
		return fmt.Errorf("failed to build sync operation for Application %s: %w", app.GetName(), err)
	}

	if err := r.Patch(ctx, app, patch); err != nil {
		return fmt.Errorf("failed to trigger sync on Application %s: %w", app.GetName(), err)
	}

	log.Info("Sync operation triggered on Application", "name", app.GetName())
	return nil
}
