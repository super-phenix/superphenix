package cluster

import (
	"context"
	"fmt"
	"maps"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kjson "k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
)

const (
	// TalosBootstrapApp is the ArgoCD Application name for the talos-bootstrap chart.
	// FIXME: The chart will be renamed to "talos-manager" later. We change it only here for now.
	TalosBootstrapApp = "talos-manager"
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

		app := r.initTalosBootstrapApp(fmt.Sprintf("%s-%s", TalosBootstrapApp, cluster.Name))

		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, app, func() error {
			// Ensure labels are up to date
			r.setTalosBootstrapAppLabels(app, cluster)

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

			// Define and set Application Spec
			spec := r.buildTalosBootstrapAppSpec(values)
			return unstructured.SetNestedMap(app.Object, spec, "spec")
		}); err != nil {
			log.Error(err, "Failed to reconcile talos-bootstrap Application")
			return err
		}

		log.Info("Successfully reconciled talos-bootstrap Application")

		return nil
	} else {
		return fmt.Errorf("Failed to reconcile talos-bootstrap Application: missing TalosBootstrapConfiguration")
	}
}

// initTalosBootstrapApp creates the template of the talos-bootstrap Application.
func (r *Reconciler) initTalosBootstrapApp(name string) *unstructured.Unstructured {
	app := &unstructured.Unstructured{}
	app.SetName(name)
	app.SetNamespace(r.OperatorNamespace)
	app.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Application",
	})

	return app
}

// setTalosBootstrapAppLabels sets the required labels on the talos-bootstrap Application.
func (r *Reconciler) setTalosBootstrapAppLabels(app *unstructured.Unstructured, cluster *operatorv1alpha1.Cluster) {
	labels := app.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[version.ManagedLabel] = "true"
	app.SetLabels(labels)
}

// buildTalosBootstrapAppSpec creates the specs of the talos-bootstrap application
func (r *Reconciler) buildTalosBootstrapAppSpec(vals map[string]any) map[string]any {
	return map[string]any{
		"project": "default",
		"source": map[string]any{
			"repoURL":        r.TalosBootstrapChartURL,
			"targetRevision": r.TalosBootstrapChartVersion,
			"chart":          "talos-bootstrap",
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
	}
}
