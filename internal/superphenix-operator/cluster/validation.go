package cluster

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
)

// validate runs all the validation logic for the cluster.
func (r *Reconciler) validate(ctx context.Context, cluster *v1alpha1.Cluster) error {
	// Validate version upgrade/downgrade
	if err := r.validateUpgradePath(ctx, cluster); err != nil {
		return err
	}

	// Validate compatibility with management cluster
	if err := r.validateManagementCompatibility(ctx, cluster); err != nil {
		return err
	}

	return nil
}

// validateUpgradePath ensures the upgrade path is possible and safe.
func (r *Reconciler) validateUpgradePath(ctx context.Context, cluster *v1alpha1.Cluster) error {
	specVersion := cluster.Spec.Version
	statusVersion := cluster.Status.SuperphenixVersion

	return version.IsClusterUpgradeSupported(statusVersion, specVersion)
}

// validateManagementCompatibility ensures the management cluster can handle the cluster version.
func (r *Reconciler) validateManagementCompatibility(ctx context.Context, cluster *v1alpha1.Cluster) error {
	mgmtVersion, err := version.GetCurrentManagementVersion(ctx, r, r.OperatorNamespace)
	if err != nil {
		return fmt.Errorf("failed to get management version: %w", err)
	}

	if mgmtVersion == "" {
		// Management version not found, maybe first install or not using the standard chart.
		return nil
	}

	return version.IsClusterCompatibleWithManagement(cluster.Spec.Version, mgmtVersion)
}
