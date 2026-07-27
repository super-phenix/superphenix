package cluster

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
)

// reconcileAppProject ensures an ArgoCD AppProject exists for each cluster.
// This project restricts deployments to only the cluster in question.
func (r *Reconciler) reconcileAppProject(ctx context.Context, cluster *operatorv1alpha1.Cluster) error {
	log := logf.FromContext(ctx)

	project := r.initAppProject(cluster)

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, project, func() error {
		// Ensure finalizers
		controllerutil.AddFinalizer(project, "resources-finalizer.argocd.argoproj.io")

		// Set ownership
		if err := r.setAppProjectOwnership(cluster, project); err != nil {
			return err
		}

		// Define and set AppProject Spec
		spec := r.buildAppProjectSpec(cluster)
		return unstructured.SetNestedMap(project.Object, spec, "spec")
	})

	if err != nil {
		log.Error(err, "Failed to reconcile ArgoCD AppProject")
		return err
	}

	log.Info("Successfully reconciled ArgoCD AppProject", "AppProject.Name", project.GetName())
	return nil
}

// initAppProject creates the template of the cluster AppProject.
func (r *Reconciler) initAppProject(cluster *operatorv1alpha1.Cluster) *unstructured.Unstructured {
	project := &unstructured.Unstructured{}
	project.SetName(cluster.Name)
	project.SetNamespace(r.OperatorNamespace)
	project.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "AppProject",
	})

	return project
}

// setAppProjectOwnership ensures the AppProject is owned by the Cluster CRD.
func (r *Reconciler) setAppProjectOwnership(cluster *operatorv1alpha1.Cluster, project *unstructured.Unstructured) error {
	return controllerutil.SetControllerReference(cluster, project, r.Scheme)
}

func (r *Reconciler) buildAppProjectSpec(cluster *operatorv1alpha1.Cluster) map[string]interface{} {
	destName := cluster.Name
	if cluster.Spec.Connection.Mode == operatorv1alpha1.ConnectionModeLocal {
		destName = "in-cluster"
	}

	destinations := []interface{}{
		map[string]interface{}{
			"namespace": "*",
			"name":      destName,
		},
	}

	// Also add the in-cluster destination for the root application (on the management cluster)
	// if it's not already there.
	if destName != "in-cluster" {
		destinations = append(destinations, map[string]interface{}{
			"namespace": r.OperatorNamespace,
			"name":      "in-cluster",
		})
	}

	spec := map[string]interface{}{
		"description":  "Project for cluster " + cluster.Name,
		"sourceRepos":  []interface{}{"*"},
		"destinations": destinations,
		"clusterResourceWhitelist": []interface{}{
			map[string]interface{}{
				"group": "*",
				"kind":  "*",
			},
		},
		"namespaceResourceWhitelist": []interface{}{
			map[string]interface{}{
				"group": "*",
				"kind":  "*",
			},
		},
	}

	if cluster.Spec.PauseSync {
		spec["syncWindows"] = []interface{}{
			map[string]interface{}{
				"kind":         "deny",
				"schedule":     "0 0 * * *", // We use a dummy schedule as we want it to be always active
				"duration":     "24h",       // Cover the whole day
				"manualSync":   false,
				"applications": []interface{}{"*"},
				"clusters":     []interface{}{destName, "in-cluster"},
				"namespaces":   []interface{}{"*"},
			},
		}
	}

	return spec
}
