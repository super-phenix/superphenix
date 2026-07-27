package cluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
)

/*
Cluster State Machine:

Phases:
- "": Initial state, set to Deploying on first reconcile.
- Deploying: Cluster is being provisioned or Superphenix stack is being deployed/updated.
- Deployed: ArgoCD Application is Synced and Health is Healthy (or at least not Degraded).
- OutOfSync: ArgoCD Application is OutOfSync.
- Error: Health check failed, ArgoCD sync failed, or ArgoCD health is Degraded.
- Paused: Synchronization is paused (PauseSync: true).
- Unknown: ArgoCD sync status is Unknown, or health is Suspended/Missing.

Transitions:
1. Any -> Deploying:
   - When Status.Phase is empty.
   - When a version update is triggered (SuperphenixVersion != Spec.Version).
2. Deploying -> Deployed:
   - When Spec.Version == SuperphenixVersion AND ArgoCD sync status is "Synced".
3. Any -> Paused:
   - When Spec.PauseSync is true.
4. Paused -> Any:
   - When Spec.PauseSync is false (returns to previous state on next reconcile).
5. Deployed -> OutOfSync:
   - When ArgoCD sync status becomes "OutOfSync".
6. Any -> Error:
   - When cluster is unreachable.
   - When ArgoCD health is "Degraded".
   - When ArgoCD sync status is anything other than Synced, OutOfSync, Syncing, or Unknown.
*/

// setCondition adds or updates a condition in the status conditions slice.
// If a condition of the same type already exists, it is updated only if the status, reason, or message has changed.
func (r *Reconciler) setCondition(conditions *[]metav1.Condition, newCondition metav1.Condition) {
	for i, c := range *conditions {
		if c.Type == newCondition.Type {
			if c.Status == newCondition.Status && c.Reason == newCondition.Reason && c.Message == newCondition.Message {
				return
			}
			(*conditions)[i] = newCondition
			return
		}
	}
	*conditions = append(*conditions, newCondition)
}

// isSynced checks if the Cluster's ArgoCDSynced condition is True.
func (r *Reconciler) isSynced(cluster *operatorv1alpha1.Cluster) bool {
	for _, c := range cluster.Status.Conditions {
		if c.Type == operatorv1alpha1.ConditionTypeArgoCDSynced && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// syncStatus centralizes the cluster status and phase management.
// It determines the final status based on the connectivity, ArgoCD application status, and spec.
// It also updates versions, conditions, phase, and handles the status patch to the Kubernetes API.
func (r *Reconciler) syncStatus(ctx context.Context, cluster *operatorv1alpha1.Cluster, app *unstructured.Unstructured, k8sVersion string, nodeCount int, reconcileErr error, lastSync *metav1.Time) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	oldStatus := cluster.Status.DeepCopy()
	patch := client.MergeFrom(cluster.DeepCopy())

	// 1. Update Versions
	if k8sVersion != "" {
		cluster.Status.KubernetesVersion = k8sVersion
	}
	if nodeCount > 0 {
		cluster.Status.NodeCount = nodeCount
	}

	// Update SuperphenixVersion based on the currently deployed superphenix-system chart
	if reconcileErr == nil {
		currentVersion, err := version.GetCurrentClusterVersion(ctx, r, cluster.Name, r.OperatorNamespace)
		if err != nil {
			log.Error(err, "Failed to get current cluster version")
		} else if currentVersion != "" {
			if cluster.Status.SuperphenixVersion != currentVersion {
				log.Info("Updating superphenix version from deployed chart", "oldVersion", cluster.Status.SuperphenixVersion, "newVersion", currentVersion)
				cluster.Status.SuperphenixVersion = currentVersion
			}
		}
	}

	// Refresh the per-app status map from the ArgoCD Applications handled by the root app.
	r.updateAppsStatus(ctx, cluster)

	// 2. Determine Conditions
	// Check version compatibility with management
	r.updateCompatibilityCondition(ctx, cluster)

	if reconcileErr != nil {
		reason := operatorv1alpha1.ReasonHealthCheckFailed
		condType := operatorv1alpha1.ConditionTypeReady

		if strings.Contains(reconcileErr.Error(), "not supported") {
			reason = operatorv1alpha1.ReasonInvalidVersion
		} else if strings.Contains(reconcileErr.Error(), "connection.secretRef is required") {
			reason = operatorv1alpha1.ReasonConnectionConfigError
			condType = operatorv1alpha1.ConditionTypeReachable
		} else if strings.Contains(reconcileErr.Error(), "invalid secret") {
			reason = operatorv1alpha1.ReasonInvalidSecret
			condType = operatorv1alpha1.ConditionTypeReachable
		} else if apierrors.IsNotFound(reconcileErr) || strings.Contains(reconcileErr.Error(), "not found") {
			// This covers both SecretNotFound and other 404s during reconciliation
			reason = operatorv1alpha1.ReasonSecretNotFound
			condType = operatorv1alpha1.ConditionTypeReachable
		} else if strings.Contains(reconcileErr.Error(), "failed to reconcile clusters ConfigMap") {
			reason = operatorv1alpha1.ReasonConfigMapReconcileFailed
		} else if strings.Contains(reconcileErr.Error(), "failed to remove cluster from configmap") {
			reason = operatorv1alpha1.ReasonConfigMapCleanupFailed
		}

		r.setCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:    condType,
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: reconcileErr.Error(),
		})

		// If it was a reachability issue, we also mark Ready as false
		if condType == operatorv1alpha1.ConditionTypeReachable {
			r.setCondition(&cluster.Status.Conditions, metav1.Condition{
				Type:    operatorv1alpha1.ConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  reason,
				Message: reconcileErr.Error(),
			})
		}
	}

	// If we have an ArgoCD application, propagate its status
	if app != nil && !cluster.Spec.PauseSync {
		r.applyApplicationStatus(cluster, app)
	}

	// Handle PauseSync condition
	if cluster.Spec.PauseSync {
		r.setCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.ConditionTypePaused,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonPaused,
			Message: "Synchronization is paused",
		})
	}

	// 3. Determine Phase
	phase := cluster.Status.Phase

	// Default to Deploying if unknown
	if phase == "" || phase == "Unknown" {
		phase = "Deploying"
	}

	// Transitions based on status
	if reconcileErr != nil {
		phase = "Error"
	} else if cluster.Spec.PauseSync {
		phase = "Paused"
	} else if r.isSynced(cluster) {
		// If Synced, we can move to Deployed
		phase = "Deployed"
	} else {
		// If not synced and not error/paused, use whatever applyApplicationStatus set
		if cluster.Status.Phase != "" {
			phase = cluster.Status.Phase
		}
	}

	// If we are "Deployed" but version mismatch, we should be "Deploying"
	if phase == "Deployed" && cluster.Status.SuperphenixVersion != cluster.Spec.Version && cluster.Status.SuperphenixVersion != "" {
		phase = "Deploying"
	}

	// Log phase change
	if cluster.Status.Phase != phase {
		log.Info("Updating Cluster phase", "oldPhase", cluster.Status.Phase, "newPhase", phase, "cluster", cluster.Name)
		cluster.Status.Phase = phase
	}

	// 4. Final Ready Condition Check
	r.updateReadyCondition(cluster, reconcileErr)

	// 5. Update ObservedGeneration and LastSync
	cluster.Status.ObservedGeneration = cluster.Generation
	if lastSync != nil {
		cluster.Status.LastSync = lastSync
	} else if oldStatus != nil {
		cluster.Status.LastSync = oldStatus.LastSync
	}

	// 6. Set LastTransitionTime for all conditions
	now := metav1.Now()
	for i := range cluster.Status.Conditions {
		cluster.Status.Conditions[i].ObservedGeneration = cluster.Generation

		// Initialize with now, will be overridden if matching old condition is found
		cluster.Status.Conditions[i].LastTransitionTime = now

		// Find existing condition to preserve LastTransitionTime
		if oldStatus != nil {
			for _, oldC := range oldStatus.Conditions {
				if oldC.Type == cluster.Status.Conditions[i].Type {
					if oldC.Status == cluster.Status.Conditions[i].Status &&
						oldC.Reason == cluster.Status.Conditions[i].Reason &&
						oldC.Message == cluster.Status.Conditions[i].Message &&
						!oldC.LastTransitionTime.IsZero() {
						cluster.Status.Conditions[i].LastTransitionTime = oldC.LastTransitionTime
					}
					break
				}
			}
		}

		// Ensure it's never zero
		if cluster.Status.Conditions[i].LastTransitionTime.IsZero() {
			cluster.Status.Conditions[i].LastTransitionTime = now
		}
	}

	// 7. Patch status
	if err := r.Status().Patch(ctx, cluster, patch); err != nil {
		log.Error(err, "Failed to patch Cluster status")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	return ctrl.Result{}, nil
}

// applyApplicationStatus propagates the status of the ArgoCD Application to the Cluster resource.
// It maps ArgoCD sync and health statuses to Cluster conditions and phases.
func (r *Reconciler) applyApplicationStatus(cluster *operatorv1alpha1.Cluster, app *unstructured.Unstructured) {
	healthStatus, _, _ := unstructured.NestedString(app.Object, "status", "health", "status")
	syncStatus, _, _ := unstructured.NestedString(app.Object, "status", "sync", "status")

	var status metav1.ConditionStatus
	var reason string
	var message string
	var phase string

	switch syncStatus {
	case "Synced":
		status = metav1.ConditionTrue
		reason = operatorv1alpha1.ReasonArgoCDSynced
		message = "ArgoCD Application is synced"
		phase = "Deployed"
	case "OutOfSync":
		status = metav1.ConditionFalse
		reason = operatorv1alpha1.ReasonArgoCDOutOfSync
		message = "ArgoCD Application is out of sync"
		phase = "OutOfSync"
	case "Unknown":
		status = metav1.ConditionUnknown
		reason = operatorv1alpha1.ReasonArgoCDUnknown
		message = "ArgoCD Application status is unknown"
		phase = "Unknown"
	case "Syncing":
		status = metav1.ConditionFalse
		reason = operatorv1alpha1.ReasonArgoCDSyncing
		message = "ArgoCD Application is syncing"
		phase = "Deploying"
	case "":
		status = metav1.ConditionUnknown
		reason = operatorv1alpha1.ReasonArgoCDUnknown
		message = "ArgoCD Application sync status is not yet available"
		phase = cluster.Status.Phase
		if phase == "" {
			phase = "Unknown"
		}
	default:
		status = metav1.ConditionFalse
		reason = operatorv1alpha1.ReasonArgoCDSyncFailed
		message = fmt.Sprintf("ArgoCD Application sync status: %s", syncStatus)
		phase = "Error"
	}

	if healthStatus == "Degraded" {
		phase = "Error"
		message = fmt.Sprintf("%s (Health: %s)", message, healthStatus)
		status = metav1.ConditionFalse
		reason = operatorv1alpha1.ReasonHealthCheckFailed
	} else if healthStatus == "Suspended" || healthStatus == "Missing" {
		phase = "Unknown"
		message = fmt.Sprintf("%s (Health: %s)", message, healthStatus)
		status = metav1.ConditionUnknown
	}

	r.setCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:    operatorv1alpha1.ConditionTypeArgoCDSynced,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
	cluster.Status.Phase = phase
}

// updateReadyCondition calculates the aggregate Ready condition for the Cluster.
// It considers reachability, ArgoCD synchronization status, compatibility, and any reconciliation errors.
func (r *Reconciler) updateReadyCondition(cluster *operatorv1alpha1.Cluster, reconcileErr error) {
	reachable := false
	var reachableCond *metav1.Condition
	for i := range cluster.Status.Conditions {
		if cluster.Status.Conditions[i].Type == operatorv1alpha1.ConditionTypeReachable {
			reachableCond = &cluster.Status.Conditions[i]
			if reachableCond.Status == metav1.ConditionTrue {
				reachable = true
			}
			break
		}
	}

	var argoSyncedCond *metav1.Condition
	for i := range cluster.Status.Conditions {
		if cluster.Status.Conditions[i].Type == operatorv1alpha1.ConditionTypeArgoCDSynced {
			argoSyncedCond = &cluster.Status.Conditions[i]
			break
		}
	}

	var compatibleCond *metav1.Condition
	for i := range cluster.Status.Conditions {
		if cluster.Status.Conditions[i].Type == operatorv1alpha1.ConditionTypeCompatibleVersion {
			compatibleCond = &cluster.Status.Conditions[i]
			break
		}
	}

	// Use existing Ready condition if set by syncStatus (e.g. for errors)
	var readyCond *metav1.Condition
	for i := range cluster.Status.Conditions {
		if cluster.Status.Conditions[i].Type == operatorv1alpha1.ConditionTypeReady {
			readyCond = &cluster.Status.Conditions[i]
			break
		}
	}

	readyStatus := metav1.ConditionTrue
	readyReason := operatorv1alpha1.ReasonReconcileSuccess
	readyMessage := "Cluster is ready"

	// If we have an error from the current reconciliation pass, use it.
	if reconcileErr != nil && readyCond != nil && readyCond.Status == metav1.ConditionFalse {
		readyStatus = readyCond.Status
		readyReason = readyCond.Reason
		readyMessage = readyCond.Message
	}

	if !reachable {
		readyStatus = metav1.ConditionFalse
		readyReason = operatorv1alpha1.ReasonConnectionFailed
		readyMessage = "Cluster is unreachable"
		if reachableCond != nil && reachableCond.Message != "" {
			readyMessage = fmt.Sprintf("Cluster is unreachable: %s", reachableCond.Message)
		}
	} else if compatibleCond != nil && compatibleCond.Status == metav1.ConditionFalse {
		readyStatus = metav1.ConditionFalse
		readyReason = compatibleCond.Reason
		readyMessage = compatibleCond.Message
	} else if argoSyncedCond != nil && argoSyncedCond.Status != metav1.ConditionTrue {
		// Only override if not already in Error/False state from reconcileErr
		if readyStatus != metav1.ConditionFalse {
			readyStatus = argoSyncedCond.Status
			readyReason = argoSyncedCond.Reason
			readyMessage = argoSyncedCond.Message
		}
	}

	r.setCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:    operatorv1alpha1.ConditionTypeReady,
		Status:  readyStatus,
		Reason:  readyReason,
		Message: readyMessage,
	})
}

// updateCompatibilityCondition ensures the cluster version is supported by the management version.
func (r *Reconciler) updateCompatibilityCondition(ctx context.Context, cluster *operatorv1alpha1.Cluster) {
	mgmtVersion, err := version.GetCurrentManagementVersion(ctx, r, r.OperatorNamespace)
	if err != nil {
		r.setCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.ConditionTypeCompatibleVersion,
			Status:  metav1.ConditionUnknown,
			Reason:  "ManagementVersionUnknown",
			Message: fmt.Sprintf("Failed to get management version: %v", err),
		})
		return
	}

	if mgmtVersion == "" {
		// Management version not found.
		r.setCondition(&cluster.Status.Conditions, metav1.Condition{
			Type:    operatorv1alpha1.ConditionTypeCompatibleVersion,
			Status:  metav1.ConditionTrue,
			Reason:  operatorv1alpha1.ReasonCompatibleVersion,
			Message: "Management version could not be determined, assuming compatible",
		})
		return
	}

	err = version.IsClusterCompatibleWithManagement(cluster.Spec.Version, mgmtVersion)
	status := metav1.ConditionTrue
	reason := operatorv1alpha1.ReasonCompatibleVersion
	message := "Cluster version is compatible with management version"
	if err != nil {
		status = metav1.ConditionFalse
		reason = operatorv1alpha1.ReasonIncompatibleVersion
		message = err.Error()
	}

	r.setCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:    operatorv1alpha1.ConditionTypeCompatibleVersion,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}

// updateAppsStatus refreshes cluster.Status.Apps with one entry per ArgoCD Application handled
// by the cluster's root app-of-apps. The root Application itself is excluded so the map reflects
// only the components it manages. Listing errors are non-fatal: the previous map is preserved so
// a transient failure does not blank out the reported state.
func (r *Reconciler) updateAppsStatus(ctx context.Context, cluster *operatorv1alpha1.Cluster) {
	log := logf.FromContext(ctx)

	subApps, err := r.listSubApplications(ctx, cluster)
	if err != nil {
		log.Error(err, "Failed to list sub-applications, preserving existing Apps status")
		return
	}

	apps := make(map[string]operatorv1alpha1.ClusterApp, len(subApps))
	for i := range subApps {
		app := &subApps[i]
		name := app.GetName()
		if name == "" || name == cluster.Name {
			continue
		}
		apps[name] = buildClusterApp(app)
	}

	if len(apps) == 0 {
		cluster.Status.Apps = nil
		return
	}
	cluster.Status.Apps = apps
}

// buildClusterApp extracts the observable fields the operator reports for a single ArgoCD
// Application. Missing or unparseable fields are left at their zero value rather than causing
// the whole entry to be dropped.
func buildClusterApp(app *unstructured.Unstructured) operatorv1alpha1.ClusterApp {
	entry := operatorv1alpha1.ClusterApp{Name: app.GetName()}

	if healthStatus, _, _ := unstructured.NestedString(app.Object, "status", "health", "status"); healthStatus != "" {
		entry.Status = healthStatus
	}

	entry.Version = applicationTargetRevision(app)

	if reconciledAt, _, _ := unstructured.NestedString(app.Object, "status", "reconciledAt"); reconciledAt != "" {
		if t, err := time.Parse(time.RFC3339, reconciledAt); err == nil {
			mt := metav1.NewTime(t)
			entry.LastRefresh = &mt
		}
	}

	if finishedAt, _, _ := unstructured.NestedString(app.Object, "status", "operationState", "finishedAt"); finishedAt != "" {
		if t, err := time.Parse(time.RFC3339, finishedAt); err == nil {
			mt := metav1.NewTime(t)
			entry.LastSync = &mt
		}
	}

	return entry
}

// applicationTargetRevision returns the target revision of the Application, handling both the
// single-source (spec.source.targetRevision) and multi-source (spec.sources[0].targetRevision)
// layouts. Returns an empty string when neither is set.
func applicationTargetRevision(app *unstructured.Unstructured) string {
	if rev, found, _ := unstructured.NestedString(app.Object, "spec", "source", "targetRevision"); found && rev != "" {
		return rev
	}
	sources, found, _ := unstructured.NestedSlice(app.Object, "spec", "sources")
	if !found || len(sources) == 0 {
		return ""
	}
	first, ok := sources[0].(map[string]interface{})
	if !ok {
		return ""
	}
	rev, _, _ := unstructured.NestedString(first, "targetRevision")
	return rev
}
