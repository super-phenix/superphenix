package cluster

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
)

// validate runs all the validation logic for the cluster.
func (r *Reconciler) validate(ctx context.Context, cluster *v1alpha1.Cluster) error {
	// Validate topology and type constraints
	if err := r.validateTopology(cluster); err != nil {
		return err
	}

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

// validateTopology ensures the cluster type is compatible with the deployment topology.
func (r *Reconciler) validateTopology(cluster *v1alpha1.Cluster) error {
	topology := cluster.Spec.DeploymentTopology
	clusterType := cluster.Spec.Type

	if topology == "" {
		if clusterType == nil || *clusterType != v1alpha1.ClusterTypeManagement {
			return fmt.Errorf("cluster type must be Management when topology is empty")
		}
	} else if topology == v1alpha1.DeploymentTopologyDecoupled {
		if clusterType == nil || (*clusterType != v1alpha1.ClusterTypeStorage && *clusterType != v1alpha1.ClusterTypeWorkload) {
			return fmt.Errorf("cluster type must be Storage or Workload when topology is Decoupled")
		}
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
