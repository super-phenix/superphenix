package cluster

import (
	"context"
	"fmt"
	"maps"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kjson "k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
)

const (
	// TalosBootstrapApp is the ArgoCD Application name for the talos-bootstrap chart.
	TalosBootstrapApp = "talos-bootstrap"
)

// reconcileTalosBootstrap creates or updates the ArgoCD Application for the talos-bootstrap chart,
// which sets up PXE on nodes and generates a TalosCluster resource for talos-operator to manage the cluster.
func (r *Reconciler) reconcileTalosBootstrap(ctx context.Context, cluster *operatorv1alpha1.Cluster) error {
	log := logf.FromContext(ctx)

	values := map[string]any{
		"pxeEnabled": true,
	}
	if cluster.Spec.TalosManagementMode == operatorv1alpha1.TalosManagementImport {
		// "Import" mode disables PXE from talos-bootstrap
		values = map[string]any{
			"pxeEnabled": false,
		}
	}
	if cluster.Spec.TalosBootstrapConfiguration != nil {
		var talosBootstrapConfig map[string]any
		// Use k8s JSON unmarshaler (PreserveInts) so integer values like port numbers
		// decode as int64 — matching what the API server returns when reading the spec back.
		// Standard encoding/json decodes all numbers as float64, which causes a type mismatch
		// in CreateOrUpdate's DeepEqual check and triggers an infinite reconcile loop.
		if err := kjson.Unmarshal(cluster.Spec.TalosBootstrapConfiguration.Raw, &talosBootstrapConfig); err == nil {
			maps.Copy(values, talosBootstrapConfig)
		} else {
			return err
		}

		app := r.buildTalosBootstrapApp(fmt.Sprintf("%s-%s", TalosBootstrapApp, cluster.Name), r.TalosBootstrapChartURL, "talos-bootstrap", r.TalosBootstrapChartVersion, values)

		// Set ownership and finalizers
		if err := r.setApplicationOwnership(cluster, app); err != nil {
			return err
		}

		// Handle finalizers based on CleanupOnDeletion
		// This finalizer propagates the deletion of the app to the resources it manages
		finalizer := "resources-finalizer.argocd.argoproj.io"
		if cluster.Spec.CleanupOnDeletion {
			controllerutil.AddFinalizer(app, finalizer)
		} else {
			controllerutil.RemoveFinalizer(app, finalizer)
		}

		if err := r.createOrUpdateArgoCDApplication(ctx, app); err != nil {
			return err
		}

		log.Info("Successfully reconciled talos-bootstrap Application")

		return nil
	} else {
		return fmt.Errorf("Failed to reconcile talos-bootstrap Application: missing TalosBootstrapConfiguration")
	}
}

// buildTalosBootstrapApp constructs a self-syncing ArgoCD Application manifest backed by a Helm chart.
func (r *Reconciler) buildTalosBootstrapApp(name, repoURL, chartName, chartVersion string, vals map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]any{
				"name":      name,
				"namespace": r.OperatorNamespace,
				"labels": map[string]any{
					version.ManagedLabel: "true",
				},
			},
			"spec": map[string]any{
				"project": "default",
				"source": map[string]any{
					"repoURL":        repoURL,
					"targetRevision": chartVersion,
					"chart":          chartName,
					"helm": map[string]any{
						"valuesObject": vals,
					},
				},
				"destination": map[string]any{
					"name":      "in-cluster",
					"namespace": r.OperatorNamespace,
				},
				"syncPolicy": map[string]any{
					"automated": map[string]any{
						"prune":    true,
						"selfHeal": true,
					},
					"syncOptions": []any{
						"RespectIgnoreDifferences=true",
					},
				},
				// The "kernelCmdlineArgs" field is modified by the job "linkaliases-setup" from the talos-bootstrap chart after deployment, so it should be ignored by ArgoCD:
				"ignoreDifferences": []any{
					map[string]any{
						"group": "talos.alperen.cloud",
						"kind":  "TalosCluster",
						"jqPathExpressions": []any{
							".spec.controlPlane.metalSpec.machines[].pxeClientSpec.kernelCmdlineArgs",
							".spec.worker.metalSpec.machines[].pxeClientSpec.kernelCmdlineArgs",
						},
					},
				},
			},
		},
	}
}

// createOrUpdateArgoCDApplication creates the ArgoCD Application if it does not exist, or updates it otherwise.
// The application name is derived from app.GetName().
func (r *Reconciler) createOrUpdateArgoCDApplication(ctx context.Context, app *unstructured.Unstructured) error {
	log := logf.FromContext(ctx)
	name := app.GetName()

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Application",
	})

	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: r.OperatorNamespace}, existing)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get ArgoCD Application %s: %w", name, err)
		}
		log.Info("Creating ArgoCD Application", "name", name)
		if err := r.Create(ctx, app); err != nil {
			return fmt.Errorf("failed to create ArgoCD Application %s: %w", name, err)
		}
		return nil
	}

	// Only update if the spec actually changed to avoid spurious generation bumps.
	existingSpec, _, _ := unstructured.NestedMap(existing.Object, "spec")
	desiredSpec, _, _ := unstructured.NestedMap(app.Object, "spec")
	if reflect.DeepEqual(existingSpec, desiredSpec) {
		log.Info("ArgoCD Application spec unchanged, skipping update", "name", name)
		return nil
	}

	log.Info("Updating ArgoCD Application", "name", name)
	app.SetResourceVersion(existing.GetResourceVersion())
	if err := r.Update(ctx, app); err != nil {
		return fmt.Errorf("failed to update ArgoCD Application %s: %w", name, err)
	}

	return nil
}
