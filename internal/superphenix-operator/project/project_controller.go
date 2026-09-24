package project

import (
	"context"
	"fmt"
	"slices"
	"strings"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/db"
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

const (
	// FinalizerName is the name of the finalizer used to clean up the project when it is deleted.
	FinalizerName = "operator.superphenix.net/finalizer"
)

// +kubebuilder:rbac:groups=operator.superphenix.net,resources=projects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.superphenix.net,resources=projects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.superphenix.net,resources=projects/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.superphenix.net,resources=organizations,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.superphenix.net,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete

// Reconciler reconciles a Project object.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// OperatorNamespace is the namespace the operator watches. When empty, all
	// namespaces are reconciled.
	OperatorNamespace string

	// GitOpsConfig is the default GitOps configuration for projects.
	GitOpsConfig GitOpsConfig
}

// GitOpsConfig defines the default GitOps configuration for projects.
type GitOpsConfig struct {
	RepoURL        string
	Path           string
	TargetRevision string
}

// SetupWithManager registers the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Project{}, builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
			},
		})).
		Named("project").
		Complete(r)
}

// Reconcile reconciles the state of a Project resource.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if r.OperatorNamespace != "" && req.Namespace != r.OperatorNamespace {
		return ctrl.Result{}, nil
	}

	proj := &operatorv1alpha1.Project{}
	if err := r.Get(ctx, req.NamespacedName, proj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log.V(1).Info("Reconciling Project", "name", proj.Name, "namespace", proj.Namespace)

	// Handle Finalizers
	stop, res, err := r.reconcileFinalizers(ctx, proj)
	if err != nil || stop {
		return res, err
	}

	// Reconcile Organization reference
	stop, res, err = r.reconcileOrganization(ctx, proj)
	if err != nil || stop {
		return res, err
	}

	// Reconcile Availability Zones
	if err := r.reconcileAvailabilityZones(ctx, proj); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Database Binding
	if err := r.reconcileDatabaseBinding(ctx, proj); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile GitOps
	if err := r.reconcileGitOps(ctx, proj); err != nil {
		log.Error(err, "Failed to reconcile GitOps")
		return ctrl.Result{}, err
	}

	proj.Status.ObservedGeneration = proj.Generation
	if err := r.Status().Update(ctx, proj); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) reconcileFinalizers(ctx context.Context, proj *operatorv1alpha1.Project) (bool, ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !proj.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(proj, FinalizerName) {
			// Clean up the ArgoCD Application and Namespace if they were created by the operator.
			projectSPXID := spxId.ToSPXID(proj.Spec.ProjectID)
			appName := fmt.Sprintf("gitops-%s", projectSPXID)

			app := &argov1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      appName,
					Namespace: projectSPXID,
				},
			}
			if err := r.Delete(ctx, app); err != nil && !apierrors.IsNotFound(err) {
				log.Error(err, "Failed to delete ArgoCD Application", "name", appName)
				return true, ctrl.Result{}, err
			}

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectSPXID,
				},
			}
			if err := r.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
				log.Error(err, "Failed to delete project namespace", "name", projectSPXID)
				return true, ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(proj, FinalizerName)
			if err := r.Update(ctx, proj); err != nil {
				return true, ctrl.Result{}, err
			}
		}
		return true, ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(proj, FinalizerName) {
		controllerutil.AddFinalizer(proj, FinalizerName)
		if err := r.Update(ctx, proj); err != nil {
			return true, ctrl.Result{}, err
		}
		return true, ctrl.Result{}, nil
	}

	return false, ctrl.Result{}, nil
}

func (r *Reconciler) reconcileOrganization(ctx context.Context, proj *operatorv1alpha1.Project) (bool, ctrl.Result, error) {
	log := logf.FromContext(ctx)
	orgNamespace := proj.Spec.OrganizationRef.Namespace
	if orgNamespace == "" {
		orgNamespace = proj.Namespace
	}
	org := &operatorv1alpha1.Organization{}
	if err := r.Get(ctx, types.NamespacedName{Name: proj.Spec.OrganizationRef.Name, Namespace: orgNamespace}, org); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Referenced Organization not found", "organization", proj.Spec.OrganizationRef.Name, "namespace", orgNamespace)
			meta.SetStatusCondition(&proj.Status.Conditions, metav1.Condition{
				Type:               operatorv1alpha1.ConditionTypeBound,
				Status:             metav1.ConditionFalse,
				Reason:             "OrganizationNotFound",
				Message:            "Referenced Organization not found",
				ObservedGeneration: proj.Generation,
			})
			if err := r.Status().Update(ctx, proj); err != nil {
				return true, ctrl.Result{}, err
			}
			return true, ctrl.Result{}, nil
		}
		return true, ctrl.Result{}, err
	}
	return false, ctrl.Result{}, nil
}

func (r *Reconciler) reconcileAvailabilityZones(ctx context.Context, proj *operatorv1alpha1.Project) error {
	var availableZones []string
	var invalidZones []string
	var notFoundZones []string

	for _, azRef := range proj.Spec.AvailabilityZones {
		azNamespace := azRef.Namespace
		if azNamespace == "" {
			azNamespace = proj.Namespace
		}
		cluster := &operatorv1alpha1.Cluster{}
		if err := r.Get(ctx, types.NamespacedName{Name: azRef.Name, Namespace: azNamespace}, cluster); err != nil {
			if apierrors.IsNotFound(err) {
				notFoundZones = append(notFoundZones, azRef.Name)
				continue
			}
			return err
		}

		isValid := false
		if cluster.Spec.DeploymentTopology == operatorv1alpha1.DeploymentTopologyHyperconverged {
			isValid = true
		} else if cluster.Spec.DeploymentTopology == operatorv1alpha1.DeploymentTopologyDecoupled &&
			cluster.Spec.Type != nil &&
			*cluster.Spec.Type == operatorv1alpha1.ClusterTypeWorkload {
			isValid = true
		}

		if isValid {
			azName := fmt.Sprintf("%s-%s", cluster.Spec.Region, cluster.Spec.AvailabilityZone)
			if !slices.Contains(availableZones, azName) {
				availableZones = append(availableZones, azName)
			}
		} else {
			invalidZones = append(invalidZones, azRef.Name)
		}
	}

	proj.Status.AvailableZones = availableZones

	if len(invalidZones) > 0 || len(notFoundZones) > 0 {
		var messages []string
		if len(notFoundZones) > 0 {
			messages = append(messages, fmt.Sprintf("Clusters not found: %s", strings.Join(notFoundZones, ", ")))
		}
		if len(invalidZones) > 0 {
			messages = append(messages, fmt.Sprintf("Clusters with invalid type: %s", strings.Join(invalidZones, ", ")))
		}

		meta.SetStatusCondition(&proj.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.ConditionTypeAvailabilityZonesReady,
			Status:             metav1.ConditionFalse,
			Reason:             operatorv1alpha1.ReasonAvailabilityZonesInvalid,
			Message:            strings.Join(messages, "; "),
			ObservedGeneration: proj.Generation,
		})
	} else {
		meta.SetStatusCondition(&proj.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.ConditionTypeAvailabilityZonesReady,
			Status:             metav1.ConditionTrue,
			Reason:             operatorv1alpha1.ReasonAvailabilityZonesValid,
			Message:            "All availability zones are valid",
			ObservedGeneration: proj.Generation,
		})
	}
	return nil
}

func (r *Reconciler) reconcileDatabaseBinding(ctx context.Context, proj *operatorv1alpha1.Project) error {
	status := metav1.ConditionFalse
	reason := operatorv1alpha1.ReasonBoundFailed
	message := "Project not found in Superphenix Database"

	if db.Client != nil {
		found, err := db.ProjectExists(proj.Spec.ProjectID)
		if err == nil && found {
			status = metav1.ConditionTrue
			reason = operatorv1alpha1.ReasonBoundFound
			message = "Project is bound to the Superphenix Database"
		} else if err != nil {
			return err
		}
	} else {
		message = "Superphenix Database connection not initialized"
	}

	meta.SetStatusCondition(&proj.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.ConditionTypeBound,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: proj.Generation,
	})
	return nil
}

func (r *Reconciler) reconcileGitOps(ctx context.Context, proj *operatorv1alpha1.Project) error {
	projectSPXID := spxId.ToSPXID(proj.Spec.ProjectID)
	namespaceName := projectSPXID
	appName := fmt.Sprintf("gitops-%s", projectSPXID)

	// Ensure Namespace exists
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ns, func() error {
		if ns.Labels == nil {
			ns.Labels = make(map[string]string)
		}
		ns.Labels["operator.superphenix.net/managed"] = "true"
		ns.Labels["operator.superphenix.net/project-id"] = proj.Spec.ProjectID
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile namespace %s: %w", namespaceName, err)
	}

	// Determine GitOps parameters
	repoURL := r.GitOpsConfig.RepoURL
	path := r.GitOpsConfig.Path
	targetRevision := r.GitOpsConfig.TargetRevision

	if proj.Spec.GitOps != nil && proj.Spec.GitOps.ManifestLocation != nil {
		if proj.Spec.GitOps.ManifestLocation.RepoURL != "" {
			repoURL = proj.Spec.GitOps.ManifestLocation.RepoURL
		}
		if proj.Spec.GitOps.ManifestLocation.Path != "" {
			path = proj.Spec.GitOps.ManifestLocation.Path
		}
		if proj.Spec.GitOps.ManifestLocation.TargetRevision != "" {
			targetRevision = proj.Spec.GitOps.ManifestLocation.TargetRevision
		}
	}

	// Ensure ArgoCD Application exists in that namespace
	app := &argov1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespaceName,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, app, func() error {
		if app.Labels == nil {
			app.Labels = make(map[string]string)
		}
		app.Labels["operator.superphenix.net/managed"] = "true"

		// Note: Standard owner references don't work across namespaces.
		// We handle cleanup in the project finalizer.

		app.Spec.Source = &argov1alpha1.ApplicationSource{
			RepoURL:        repoURL,
			Path:           path,
			TargetRevision: targetRevision,
		}
		app.Spec.Destination = argov1alpha1.ApplicationDestination{
			Server:    "https://kubernetes.default.svc",
			Namespace: namespaceName,
		}
		app.Spec.Project = "default"
		app.Spec.SyncPolicy = &argov1alpha1.SyncPolicy{
			Automated: &argov1alpha1.SyncPolicyAutomated{
				Prune:    true,
				SelfHeal: true,
			},
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile argo application %s: %w", appName, err)
	}

	return nil
}
