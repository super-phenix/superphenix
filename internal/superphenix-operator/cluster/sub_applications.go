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

// ArgoCD OperationPhase values (status.operationState.phase).
const (
	operationPhaseRunning     = "Running"
	operationPhaseTerminating = "Terminating"
)

// runPeriodicSync triggers a sync on the root Application and every child Application of the
// cluster, at most once per SyncPeriod, so drift is corrected without relying on ArgoCD's own polling.
//
// Nothing is triggered while an operation is in progress. An operation Running for longer than
// SyncTimeout is asked to terminate; the fresh sync starts on a later reconcile, once ArgoCD has
// finished terminating it. Writing a new operation earlier would be discarded: ArgoCD clears
// .operation when the terminated operation completes.
//
// Returns true if a sync was triggered so the caller can persist LastSync.
func (r *Reconciler) runPeriodicSync(ctx context.Context, cluster *operatorv1alpha1.Cluster) bool {
	log := logf.FromContext(ctx)

	if cluster.Status.LastSync != nil && time.Since(cluster.Status.LastSync.Time) < r.SyncPeriod {
		log.Info("Skipping periodic sync, last sync was recent",
			"lastSync", cluster.Status.LastSync.Time, "syncPeriod", r.SyncPeriod)
		return false
	}

	apps, err := r.listAllApplications(ctx, cluster)
	if err != nil {
		log.Error(err, "Failed to list Applications, skipping periodic sync")
		return false
	}

	busy := false
	var stuck []*unstructured.Unstructured
	for i := range apps {
		phase, _, _ := unstructured.NestedString(apps[i].Object, "status", "operationState", "phase")
		switch phase {
		case operationPhaseTerminating:
			busy = true
		case operationPhaseRunning:
			busy = true
			// An unparseable startedAt is treated as "just started" to avoid spurious aborts.
			startedAtStr, _, _ := unstructured.NestedString(apps[i].Object, "status", "operationState", "startedAt")
			if startedAt, err := time.Parse(time.RFC3339, startedAtStr); err == nil && time.Since(startedAt) >= r.SyncTimeout {
				stuck = append(stuck, &apps[i])
			}
		}
	}

	for _, app := range stuck {
		if err := r.terminateOperation(ctx, app); err != nil {
			log.Error(err, "Failed to terminate stale sync", "name", app.GetName())
			continue
		}
		log.Info("Requested termination of stale sync", "name", app.GetName(), "syncTimeout", r.SyncTimeout)
	}
	if busy {
		log.Info("Sync in progress, skipping periodic sync", "cluster", cluster.Name)
		return false
	}

	synced := 0
	for i := range apps {
		if err := r.triggerApplicationSync(ctx, &apps[i]); err != nil {
			log.Error(err, "Failed to trigger sync", "name", apps[i].GetName())
			continue
		}
		synced++
	}
	log.Info("Periodic sync triggered", "cluster", cluster.Name, "appsSynced", synced, "appsTotal", len(apps))
	return true
}

// listAllApplications returns the root Application followed by the cluster's child Applications.
// A missing root Application is non-fatal.
func (r *Reconciler) listAllApplications(ctx context.Context, cluster *operatorv1alpha1.Cluster) ([]unstructured.Unstructured, error) {
	subApps, err := r.listSubApplications(ctx, cluster)
	if err != nil {
		return nil, err
	}
	rootApp, err := r.fetchApplication(ctx, cluster.Name)
	if err != nil {
		return subApps, nil
	}
	return append([]unstructured.Unstructured{*rootApp}, subApps...), nil
}

// terminateOperation sets status.operationState.phase to Terminating, as `argocd app terminate-op`
// does. .operation is kept: ArgoCD terminates the operation and clears it itself. The Application
// CRD has no status subresource, so status is patched on the main resource. The optimistic lock
// fails the patch if ArgoCD updated the Application since it was listed (e.g. the operation finished).
func (r *Reconciler) terminateOperation(ctx context.Context, app *unstructured.Unstructured) error {
	patch := client.MergeFromWithOptions(app.DeepCopy(), client.MergeFromWithOptimisticLock{})
	if err := unstructured.SetNestedField(app.Object, operationPhaseTerminating, "status", "operationState", "phase"); err != nil {
		return err
	}
	return r.Patch(ctx, app, patch)
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

// triggerApplicationSync sets .operation.sync on the Application, the programmatic equivalent of
// `argocd app sync`. ArgoCD runs the sync and clears the field. .operation is outside .spec, so the
// patch does not bump the generation.
func (r *Reconciler) triggerApplicationSync(ctx context.Context, app *unstructured.Unstructured) error {
	patch := client.MergeFrom(app.DeepCopy())
	if err := unstructured.SetNestedMap(app.Object, map[string]interface{}{
		"sync": map[string]interface{}{},
	}, "operation"); err != nil {
		return fmt.Errorf("failed to build sync operation for Application %s: %w", app.GetName(), err)
	}
	if err := r.Patch(ctx, app, patch); err != nil {
		return fmt.Errorf("failed to trigger sync on Application %s: %w", app.GetName(), err)
	}
	return nil
}
