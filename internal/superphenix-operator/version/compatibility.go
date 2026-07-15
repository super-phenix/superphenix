package version

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ManagementSuperphenixName is the ArgoCD Application name for the superphenix-management chart.
	ManagementSuperphenixName = "superphenix-management"

	// ClusterSystemChartName is the default name of the cluster system chart.
	ClusterSystemChartName = "superphenix-system"
)

var (
	// ClusterLabel is the label used to identify the cluster in ArgoCD.
	ClusterLabel = "operator.superphenix.net/clusterName"

	// ManagedLabel identifies Applications managed by the Superphenix operator
	// (root cluster Apps, superphenix-system child Apps, and management Apps).
	// It is used to scope the Application informer.
	ManagedLabel = "operator.superphenix.net/managed"

	// RootApplicationLabel is the label used to identify root applications in ArgoCD.
	RootApplicationLabel = "operator.superphenix.net/root"
)

var (
	// MinManagementVersionBeforeUpgrade is the minimum version the management must be in before upgrade.
	MinManagementVersionBeforeUpgrade = "0.0.0"

	// MinClusterVersion is the minimum version of the cluster supported by the operator.
	MinClusterVersion = "0.0.0"

	// MaxClusterVersion is the ceiling for supported cluster versions (exclusive).
	MaxClusterVersion = "999.999.999"

	// OperatorVersion is the version of the running operator. Overridden at build time with -ldflags.
	OperatorVersion = "dev"
)

// IsManagementUpgradeSupported checks if upgrading management from current to target version is supported.
// Upgrades are supported if the current version is at least MinManagementVersionBeforeUpgrade.
// Special versions like "0.0.0" or empty versions bypass the check.
func IsManagementUpgradeSupported(current, target string) error {
	if current == "" || current == "0.0.0" || target == "0.0.0" {
		return nil
	}

	constraintString := ">= " + MinManagementVersionBeforeUpgrade
	return checkConstraint(current, current, target, constraintString, "management")
}

// IsClusterUpgradeSupported checks if upgrading a cluster from current to target is supported.
// Upgrades are supported if both the current and target versions are within the supported range [MinClusterVersion, MaxClusterVersion[.
// Special versions like "0.0.0" or empty versions bypass the check.
func IsClusterUpgradeSupported(current, target string) error {
	if current == "" || target == "" || current == target {
		return nil
	}

	// Special case for latest
	if target == "0.0.0" {
		return nil
	}

	constraintString := fmt.Sprintf(">= %s, < %s", MinClusterVersion, MaxClusterVersion)
	if err := checkConstraint(current, current, target, constraintString, "cluster"); err != nil {
		return err
	}

	return checkConstraint(target, current, target, constraintString, "cluster")
}

// IsClusterCompatibleWithManagement checks if a cluster version is supported by a management version.
// A cluster is considered compatible if its version is within the supported range [MinClusterVersion, MaxClusterVersion[.
// Empty management version bypasses the check.
func IsClusterCompatibleWithManagement(clusterVersion, managementVersion string) error {
	if managementVersion == "" {
		return nil
	}

	constraintString := fmt.Sprintf(">= %s, < %s", MinClusterVersion, MaxClusterVersion)

	if clusterVersion == "" || clusterVersion == "0.0.0" {
		if clusterVersion == "0.0.0" {
			return checkConstraint("0.0.0", clusterVersion, managementVersion, constraintString, "cluster compatibility")
		}
		return nil
	}

	return checkConstraint(clusterVersion, clusterVersion, managementVersion, constraintString, "cluster compatibility")
}

// GetCurrentManagementVersion attempts to retrieve the current version of the management chart
// by looking at the existing ArgoCD Application in the given namespace.
// Returns an empty string if the application is not found or the version cannot be determined.
func GetCurrentManagementVersion(ctx context.Context, c client.Reader, operatorNamespace string) (string, error) {
	return GetApplicationVersion(ctx, c, ManagementSuperphenixName, operatorNamespace)
}

// GetCurrentClusterVersion attempts to retrieve the current version of the cluster
// by looking at its root ArgoCD Application.
// Returns an empty string if the application is not found or the version cannot be determined.
func GetCurrentClusterVersion(ctx context.Context, c client.Reader, clusterName, operatorNamespace string) (string, error) {
	return GetApplicationVersion(ctx, c, clusterName, operatorNamespace)
}

// GetApplicationVersion attempts to retrieve the version of a given ArgoCD Application.
func GetApplicationVersion(ctx context.Context, c client.Reader, name, namespace string) (string, error) {
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Application",
	})

	err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, app)
	if err != nil {
		if errors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return "", nil
		}
		return "", err
	}

	// The version is stored in spec.source.targetRevision
	version, found, err := unstructured.NestedString(app.Object, "spec", "source", "targetRevision")
	if err != nil || !found {
		return "", nil
	}

	return version, nil
}

func checkConstraint(vStr, current, target, constraintString, scope string) error {
	v, err := semver.NewVersion(vStr)
	if err != nil {
		return fmt.Errorf("invalid %s version %q: %w", scope, vStr, err)
	}

	constraint, err := semver.NewConstraint(constraintString)
	if err != nil {
		return fmt.Errorf("invalid semver constraint for %s version %s: %w", scope, target, err)
	}

	if !constraint.Check(v) {
		if scope == "cluster compatibility" {
			return fmt.Errorf("cluster version %s is not supported by management version %s (required: %s)",
				current, target, constraintString)
		}
		return fmt.Errorf("%s upgrade from %s to %s is not supported (must satisfy: %s)",
			scope, current, target, constraintString)
	}

	return nil
}
