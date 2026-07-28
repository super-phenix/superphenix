package cluster

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/pkg/argocd"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

const (
	// FinalizerName is the name of the finalizer used to clean up the cluster when it is deleted.
	FinalizerName = "operator.superphenix.net/finalizer"
)

var (
	errConfig        = fmt.Errorf("configuration error")
	errInvalidSecret = fmt.Errorf("invalid secret")
)

// +kubebuilder:rbac:groups=operator.superphenix.net,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.superphenix.net,resources=clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.superphenix.net,resources=clusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=argoproj.io,resources=applications;appprojects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="*",resources="*",verbs="*"

// Reconciler reconciles a Cluster object.
type Reconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	OperatorNamespace     string
	ClustersConfigMapName string
	DefaultRepoURL        string
	DefaultChartName      string
	DefaultVersion        string
	SyncPeriod            time.Duration
	SyncTimeout           time.Duration

	// ArgoCDWatcher handles dynamic watching of ArgoCD Applications.
	ArgoCDWatcher *argocd.Watcher

	// talos-manager chart configuration.
	TalosManagerChartURL     string
	TalosManagerChartVersion string
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Cluster{}, builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				// Only reconcile if the generation has changed
				// (e.g. spec changes, labels, annotations)
				// This avoids reconciliation loops when the status is updated.
				return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
			},
		})).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findClustersForSecret),
		).
		Named("cluster").
		Build(r)
	if err != nil {
		return err
	}
	r.ArgoCDWatcher = argocd.NewWatcher(r.Client, r.Scheme, r.RESTMapper(), c, mgr.GetCache())
	return nil
}

func (r *Reconciler) findClustersForSecret(ctx context.Context, secret client.Object) []reconcile.Request {
	clusterList := &operatorv1alpha1.ClusterList{}
	err := r.List(ctx, clusterList)
	if err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, cluster := range clusterList.Items {
		// Only consider clusters that are in the namespace of the controller
		if r.OperatorNamespace != "" && cluster.Namespace != r.OperatorNamespace {
			continue
		}

		if cluster.Spec.Connection != nil && cluster.Spec.Connection.SecretRef != nil {
			secretName := cluster.Spec.Connection.SecretRef.Name
			secretNamespace := cluster.Spec.Connection.SecretRef.Namespace
			if secretNamespace == "" {
				secretNamespace = cluster.Namespace
			}

			if secretName == secret.GetName() && secretNamespace == secret.GetNamespace() {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      cluster.Name,
						Namespace: cluster.Namespace,
					},
				})
			}
		}
	}
	return requests
}

// Reconcile is used to reconcile the state of Superphenix clusters with their definition.
// It is called on creations, updates, deletions, and re-queuing.
// This function handles retrieving the cluster object and the finalizer logic.
// It then defers the actual reconciliation/cleanup to other functions.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Only sync clusters that are in the namespace of the controller
	if r.OperatorNamespace != "" && req.Namespace != r.OperatorNamespace {
		return ctrl.Result{}, nil
	}

	// Fetch the Cluster instance
	cluster := &operatorv1alpha1.Cluster{}
	err := r.Get(ctx, req.NamespacedName, cluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Examine DeletionTimestamp to determine if the cluster is under deletion
	if !cluster.ObjectMeta.DeletionTimestamp.IsZero() {
		// The cluster is being deleted and the finalizer is present, so clean it up
		if controllerutil.ContainsFinalizer(cluster, FinalizerName) {
			if err := r.cleanupCluster(ctx, cluster); err != nil {
				// Update status with the cleanup error
				if _, syncErr := r.syncStatus(ctx, cluster, nil, "", 0, nil, err, nil); syncErr != nil {
					logf.FromContext(ctx).Error(syncErr, "Failed to update status after cleanup failure")
				}
				return ctrl.Result{RequeueAfter: time.Minute}, err
			}

			controllerutil.RemoveFinalizer(cluster, FinalizerName)
			if err := r.Update(ctx, cluster); err != nil {
				return ctrl.Result{RequeueAfter: time.Minute}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Add the finalizer if it doesn't exist to prevent the cluster from being deleted without cleaning up
	if !controllerutil.ContainsFinalizer(cluster, FinalizerName) {
		controllerutil.AddFinalizer(cluster, FinalizerName)
		if err := r.Update(ctx, cluster); err != nil {
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
	}

	// Handle the reconciling logic for the cluster
	return r.reconcileCluster(ctx, cluster)
}

// reconcileCluster checks if the cluster can be reached and administered and then deploys
// the Superphenix stack on it. The logic is run every 5 minutes to address runtime drifts
// and re-check if the cluster can still be reached.
func (r *Reconciler) reconcileCluster(ctx context.Context, cluster *operatorv1alpha1.Cluster) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var reconcileErr error
	var k8sVersion string
	var nodeCount int
	var cephClusters map[string]apiextensionsv1.JSON
	var app *unstructured.Unstructured

	// Reconcile ArgoCD connection secret
	if err := r.reconcileArgoCDSecret(ctx, cluster); err != nil {
		log.Error(err, "Failed to reconcile ArgoCD connection secret")
		reconcileErr = err
	}

	if reconcileErr == nil {
		// Verify the cluster can be reached and administered
		var result ctrl.Result
		k8sVersion, nodeCount, cephClusters, result, reconcileErr = r.reconcileHealth(ctx, cluster)
		if reconcileErr == nil && !result.IsZero() {
			// Health check wants to requeue without error
			return result, nil
		}
	}

	// Even if the cluster is unreachable or secret fails, we still try to reconcile the ArgoCD AppProject and Application
	// so that they are created/updated with the correct destination.
	// This is useful when the cluster is not yet reachable but we want to prepare the ArgoCD resources.
	if err := r.reconcileAppProject(ctx, cluster); err != nil {
		log.Error(err, "Failed to reconcile ArgoCD AppProject")
		if reconcileErr == nil {
			reconcileErr = err
		}
	}

	// Examination of Connection mode is used to decide the destination in ArgoCD
	// We must ensure we use the latest Spec from the cluster object passed to us.
	var err error
	app, err = r.reconcileApplication(ctx, cluster)
	if err != nil {
		log.Error(err, "Failed to reconcile ArgoCD Application")
		if reconcileErr == nil {
			reconcileErr = err
		}
	} else {
		// Fetch the latest app object after reconcile (it might have status now)
		_ = r.Get(ctx, types.NamespacedName{Name: app.GetName(), Namespace: app.GetNamespace()}, app)
	}

	if reconcileErr == nil {
		// Validate cluster configuration
		if err := r.validate(ctx, cluster); err != nil {
			reconcileErr = err
		}
	}

	if reconcileErr == nil {
		// Reconcile the cluster configuration in the shared ConfigMap
		if err := r.reconcileClustersConfigMap(ctx, cluster); err != nil {
			log.Error(err, "Failed to reconcile clusters ConfigMap")
			reconcileErr = fmt.Errorf("failed to reconcile clusters ConfigMap: %w", err)
		}
	}

	// Try to start the ArgoCD Application watch if not already started.
	r.ArgoCDWatcher.EnsureWatch(ctx, &operatorv1alpha1.Cluster{})

	// Centralized status sync
	res, err := r.syncStatus(ctx, cluster, app, k8sVersion, nodeCount, cephClusters, reconcileErr, nil)
	if err != nil || !res.IsZero() {
		return res, err
	}

	if !cluster.Spec.PauseSync && !cluster.Spec.Manual {
		// Periodically resync sub-applications to address drifts.
		// We resync if:
		// - There is no error (normal operation)
		// - The "Ready" condition is False (ArgoCD drift, health issues, or connectivity errors)
		ready := false
		for _, c := range cluster.Status.Conditions {
			if c.Type == operatorv1alpha1.ConditionTypeReady && c.Status == metav1.ConditionTrue {
				ready = true
				break
			}
		}

		if reconcileErr == nil || !ready {
			if r.runPeriodicSync(ctx, cluster) {
				now := metav1.Now()
				if _, err := r.syncStatus(ctx, cluster, app, k8sVersion, nodeCount, cephClusters, reconcileErr, &now); err != nil {
					log.Error(err, "Failed to update LastSync in status")
				}
			}
		}
	}

	// "Unmanaged" mode disables talos-manager entirely:
	if cluster.Spec.TalosManagementMode != operatorv1alpha1.TalosManagementUnmanaged {
		// Reconcile talos-manager application:
		if err := r.reconcileTalosManager(ctx, cluster); err != nil {
			log.Error(err, "talos-manager reconciliation failed")
			reconcileErr = err
		}
	} else {
		// We have to check if there is an existing talos-manager Application, because in that case, it should be deleted:
		name := fmt.Sprintf("%s-%s", TalosManagerApp, cluster.Name)
		exists := true

		existing := &unstructured.Unstructured{}
		existing.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "argoproj.io",
			Version: "v1alpha1",
			Kind:    "Application",
		})

		if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: r.OperatorNamespace}, existing); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "failed to get ArgoCD Application %s", name)
				reconcileErr = err
			} else {
				exists = false
			}
		}

		if exists && reconcileErr == nil {
			// Application exists, deleting it:
			if err := r.Delete(ctx, existing); err != nil {
				log.Error(err, "failed to delete ArgoCD Application %s", name)
				reconcileErr = err
			}
		}
	}

	if reconcileErr != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Reconcile again in the specified sync period to ensure the cluster stays in sync
	return ctrl.Result{RequeueAfter: r.SyncPeriod}, nil
}
