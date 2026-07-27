package management

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
)

// validateManagementUpgrade verifies that the management chart can be upgraded to the target version
// and that all existing clusters are compatible with this new version.
func (r *Reconciler) validateManagementUpgrade(ctx context.Context, targetVersion string) error {
	if err := r.validateManagementUpgradePath(ctx, targetVersion); err != nil {
		return err
	}
	return r.validateClustersCompatibility(ctx, targetVersion)
}

// validateManagementUpgradePath ensures that the upgrade from the current version to the target version is supported.
func (r *Reconciler) validateManagementUpgradePath(ctx context.Context, targetVersion string) error {
	// Try to get the current version from the existing ArgoCD Application.
	currentVersion, err := version.GetCurrentManagementVersion(ctx, r, r.OperatorNamespace)
	if err != nil {
		return fmt.Errorf("failed to get current management version: %w", err)
	}

	return version.IsManagementUpgradeSupported(currentVersion, targetVersion)
}

// validateClustersCompatibility ensures all Cluster resources are at a version supported by the target management version.
func (r *Reconciler) validateClustersCompatibility(ctx context.Context, managementVersion string) error {
	clusterList := &v1alpha1.ClusterList{}
	if err := r.List(ctx, clusterList); err != nil {
		return fmt.Errorf("failed to list clusters for compatibility check: %w", err)
	}

	anyIncompatible := false
	var incompatibleErrors []string
	for _, cluster := range clusterList.Items {
		clusterVersionStr := cluster.Status.SuperphenixVersion
		if clusterVersionStr == "" {
			// Fall back to spec version when the cluster has not been deployed yet.
			clusterVersionStr = cluster.Spec.Version
		}

		err := version.IsClusterCompatibleWithManagement(clusterVersionStr, managementVersion)
		status := metav1.ConditionTrue
		reason := v1alpha1.ReasonCompatibleVersion
		message := fmt.Sprintf("Cluster version is compatible with management version %s", managementVersion)
		if err != nil {
			status = metav1.ConditionFalse
			reason = v1alpha1.ReasonIncompatibleVersion
			message = err.Error()
			anyIncompatible = true
			incompatibleErrors = append(incompatibleErrors, err.Error())
		}

		// Update compatibility condition on the cluster.
		// We use a patch to avoid overwriting other status fields.
		patchBase := cluster.DeepCopy()
		r.setClusterCondition(&cluster, metav1.Condition{
			Type:    v1alpha1.ConditionTypeCompatibleVersion,
			Status:  status,
			Reason:  reason,
			Message: message,
		})

		if err := r.Status().Patch(ctx, &cluster, client.MergeFrom(patchBase)); err != nil {
			return fmt.Errorf("failed to update compatibility condition for cluster %s: %w", cluster.Name, err)
		}
	}

	if anyIncompatible {
		return fmt.Errorf("some clusters are incompatible with management version %s: %s", managementVersion, strings.Join(incompatibleErrors, "; "))
	}

	return nil
}

// setClusterCondition adds or updates a condition in the cluster status.
func (r *Reconciler) setClusterCondition(cluster *v1alpha1.Cluster, newCondition metav1.Condition) {
	newCondition.ObservedGeneration = cluster.Generation
	newCondition.LastTransitionTime = metav1.Now()

	for i, c := range cluster.Status.Conditions {
		if c.Type == newCondition.Type {
			if c.Status == newCondition.Status && c.Reason == newCondition.Reason && c.Message == newCondition.Message {
				return
			}
			// Preserve LastTransitionTime if status didn't change (optional but good practice)
			if c.Status == newCondition.Status {
				newCondition.LastTransitionTime = c.LastTransitionTime
			}
			cluster.Status.Conditions[i] = newCondition
			return
		}
	}
	cluster.Status.Conditions = append(cluster.Status.Conditions, newCondition)
}
