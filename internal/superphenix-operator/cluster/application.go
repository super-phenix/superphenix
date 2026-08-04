package cluster

import (
	"context"
	"maps"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kjson "k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
)

// reconcileApplication ensures an ArgoCD Application exists for each cluster.
// This application is used as the root of all the deployments done on each cluster.
// It uses the App of Apps pattern to deploy in cascade the entire Superphenix stack.
func (r *Reconciler) reconcileApplication(ctx context.Context, cluster *operatorv1alpha1.Cluster) (*unstructured.Unstructured, error) {
	log := logf.FromContext(ctx)

	app := r.initApplication(cluster)

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, app, func() error {
		// Ensure labels are up to date
		r.setApplicationLabels(app, cluster)

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
		spec := r.buildApplicationSpec(cluster)
		return unstructured.SetNestedMap(app.Object, spec, "spec")
	})

	if err != nil {
		log.Error(err, "Failed to reconcile ArgoCD Application")
		return nil, err
	}

	log.Info("Successfully reconciled ArgoCD Application", "Application.Name", app.GetName())

	return app, nil
}

// initApplication creates the template of the cluster application.
func (r *Reconciler) initApplication(cluster *operatorv1alpha1.Cluster) *unstructured.Unstructured {
	app := &unstructured.Unstructured{}
	app.SetName(cluster.Name)
	app.SetNamespace(r.OperatorNamespace)
	app.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Application",
	})

	return app
}

// setApplicationLabels sets the required labels on the ArgoCD Application.
func (r *Reconciler) setApplicationLabels(app *unstructured.Unstructured, cluster *operatorv1alpha1.Cluster) {
	labels := app.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[version.ClusterLabel] = cluster.Name
	labels[version.ManagedLabel] = "true"
	labels[version.RootApplicationLabel] = "true"
	app.SetLabels(labels)
}

// setApplicationOwnership ensures the application is owned by the Cluster CRD.
func (r *Reconciler) setApplicationOwnership(cluster *operatorv1alpha1.Cluster, app *unstructured.Unstructured) error {
	// Set the ownership reference back to the Cluster CRD
	if err := controllerutil.SetControllerReference(cluster, app, r.Scheme); err != nil {
		return err
	}

	return nil
}

// buildApplicationSpec creates the specs of the cluster application.
func (r *Reconciler) buildApplicationSpec(cluster *operatorv1alpha1.Cluster) map[string]interface{} {
	repoURL := r.SystemChartURL
	if cluster.Spec.RepoURL != "" {
		repoURL = cluster.Spec.RepoURL
	}

	chartName := r.SystemChartName
	if cluster.Spec.ChartName != "" {
		chartName = cluster.Spec.ChartName
	}

	targetRevision := r.SystemChartVersion
	if cluster.Spec.Version != "" {
		targetRevision = cluster.Spec.Version
	}

	syncPolicy := map[string]interface{}{
		"automated": map[string]interface{}{
			"enabled":  !cluster.Spec.PauseSync && !cluster.Spec.Manual,
			"prune":    true,
			"selfHeal": true,
		},
		"syncOptions": []interface{}{
			"CreateNamespace=true",
			"PrunePropagationPolicy=foreground",
			"PruneLast=true",
			"SkipDryRunOnMissingResource=true",
		},
		"retry": map[string]interface{}{
			"limit": int64(5),
			"backoff": map[string]interface{}{
				"duration":    "30s",
				"factor":      int64(2),
				"maxDuration": "3m",
			},
		},
	}

	return map[string]interface{}{
		"project": cluster.Name,
		"source": map[string]interface{}{
			"repoURL":        repoURL,
			"chart":          chartName,
			"targetRevision": targetRevision,
			"helm": map[string]interface{}{
				"valuesObject": r.generateApplicationValues(cluster),
			},
		},
		"destination": map[string]interface{}{
			"name":      "in-cluster",
			"namespace": r.OperatorNamespace,
		},
		"syncPolicy": syncPolicy,
	}
}

// generateApplicationValues generates the values for the cluster application Helm chart.
func (r *Reconciler) generateApplicationValues(cluster *operatorv1alpha1.Cluster) map[string]interface{} {
	values := map[string]interface{}{
		"cluster": map[string]interface{}{
			"name":               cluster.Name,
			"region":             cluster.Spec.Region,
			"availabilityZone":   cluster.Spec.AvailabilityZone,
			"deploymentTopology": string(cluster.Spec.DeploymentTopology),
		},
		"argocd": map[string]interface{}{
			"namespace": r.OperatorNamespace,
			"project":   cluster.Name,
		},
		"forceManual":       cluster.Spec.Manual,
		"cleanupOnDeletion": cluster.Spec.CleanupOnDeletion,
	}

	if cluster.Spec.Type != nil {
		values["cluster"].(map[string]interface{})["type"] = string(*cluster.Spec.Type)
	}

	version := r.SystemChartVersion
	if cluster.Spec.Version != "" {
		version = cluster.Spec.Version
	}
	values["cluster"].(map[string]interface{})["version"] = version

	if cluster.Spec.SystemConfiguration != nil {
		var systemConfig map[string]interface{}
		// Use k8s JSON unmarshaler (PreserveInts) so integer values like port numbers
		// decode as int64 — matching what the API server returns when reading the spec back.
		// Standard encoding/json decodes all numbers as float64, which causes a type mismatch
		// in CreateOrUpdate's DeepEqual check and triggers an infinite reconcile loop.
		if err := kjson.Unmarshal(cluster.Spec.SystemConfiguration.Raw, &systemConfig); err == nil {
			maps.Copy(values, systemConfig)
		}
	}

	return values
}
