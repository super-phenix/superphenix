package project

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"gopkg.in/yaml.v3"
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
	// It is also set on the project's repository Secret so the Secret outlives the ArgoCD Application.
	FinalizerName             = "operator.superphenix.net/finalizer"
	repositorySecretTypeLabel = "argocd.argoproj.io/secret-type"
	argocdResourcesFinalizer  = "resources-finalizer.argocd.argoproj.io"
)

// +kubebuilder:rbac:groups=operator.superphenix.net,resources=projects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.superphenix.net,resources=projects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.superphenix.net,resources=projects/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.superphenix.net,resources=organizations,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.superphenix.net,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

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

	// Check if Project is Bound
	boundCond := meta.FindStatusCondition(proj.Status.Conditions, operatorv1alpha1.ConditionTypeBound)
	if boundCond == nil || boundCond.Status != metav1.ConditionTrue {
		log.Info("Project is not bound to the database, skipping business logic", "name", proj.Name)
		setGitOpsReady(proj, metav1.ConditionFalse, operatorv1alpha1.ReasonGitOpsProjectNotBound, "Project is not bound to the Superphenix Database")
		proj.Status.ObservedGeneration = proj.Generation
		proj.Status.DisplayName = proj.Name
		if proj.Spec.Name != "" {
			proj.Status.DisplayName = proj.Spec.Name
		}
		if err := r.Status().Update(ctx, proj); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Reconcile GitOps
	if err := r.reconcileGitOps(ctx, proj); err != nil {
		log.Error(err, "Failed to reconcile GitOps")
		// Surface the failure reason set by reconcileGitOps.
		if statusErr := r.Status().Update(ctx, proj); statusErr != nil {
			log.Error(statusErr, "Failed to update Project status")
		}
		return ctrl.Result{}, err
	}

	proj.Status.ObservedGeneration = proj.Generation
	proj.Status.DisplayName = proj.Name
	if proj.Spec.Name != "" {
		proj.Status.DisplayName = proj.Spec.Name
	}
	if err := r.Status().Update(ctx, proj); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) reconcileFinalizers(ctx context.Context, proj *operatorv1alpha1.Project) (bool, ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !proj.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(proj, FinalizerName) {
			return true, ctrl.Result{}, nil
		}

		reason, message, err := r.cleanupGitOps(ctx, proj)
		if reason != "" {
			log.Info("Project deletion pending", "reason", reason, "message", message)
			changed := setGitOpsReady(proj, metav1.ConditionFalse, operatorv1alpha1.ReasonGitOpsProjectDeleting, "Project is being deleted")
			changed = meta.SetStatusCondition(&proj.Status.Conditions, metav1.Condition{
				Type:               operatorv1alpha1.ConditionTypeDeleting,
				Status:             metav1.ConditionTrue,
				Reason:             reason,
				Message:            message,
				ObservedGeneration: proj.Generation,
			}) || changed
			if changed {
				if statusErr := r.Status().Update(ctx, proj); statusErr != nil {
					log.Error(statusErr, "Failed to update Project status")
				}
			}
			if err != nil {
				return true, ctrl.Result{}, err
			}
			return true, ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}

		controllerutil.RemoveFinalizer(proj, FinalizerName)
		if err := r.Update(ctx, proj); err != nil {
			return true, ctrl.Result{}, err
		}
		return true, ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(proj, FinalizerName) {
		controllerutil.AddFinalizer(proj, FinalizerName)
		if err := r.Update(ctx, proj); err != nil {
			return true, ctrl.Result{}, err
		}
		return false, ctrl.Result{}, nil
	}

	return false, ctrl.Result{}, nil
}

// cleanupGitOps deletes the project's ArgoCD Application, then, once it is gone, its repository Secret.
// It returns an empty reason once both are confirmed gone; otherwise the Deleting condition reason and
// message describing what deletion is waiting on, and the error if a step failed.
func (r *Reconciler) cleanupGitOps(ctx context.Context, proj *operatorv1alpha1.Project) (string, string, error) {
	projectSPXID := spxId.ToSPXID(proj.Spec.ProjectID)
	appKey := types.NamespacedName{Name: fmt.Sprintf("gitops-%s", projectSPXID), Namespace: projectSPXID}

	app := &argov1alpha1.Application{}
	if err := r.Get(ctx, appKey, app); err == nil {
		if app.DeletionTimestamp.IsZero() {
			if err := r.Delete(ctx, app); client.IgnoreNotFound(err) != nil {
				return operatorv1alpha1.ReasonApplicationDeletionFailed,
					fmt.Sprintf("Failed to delete ArgoCD Application %s: %v", appKey, err), err
			}
		}
		message := fmt.Sprintf("Waiting for ArgoCD Application %s to be deleted", appKey)
		for _, cond := range app.Status.Conditions {
			if cond.Type == argov1alpha1.ApplicationConditionDeletionError {
				message += ": " + cond.Message
			}
		}
		return operatorv1alpha1.ReasonWaitingForApplication, message, nil
	} else if !apierrors.IsNotFound(err) {
		return operatorv1alpha1.ReasonApplicationDeletionFailed,
			fmt.Sprintf("Failed to get ArgoCD Application %s: %v", appKey, err), err
	}

	// Only release the repository Secret once the Application is gone.
	remaining, err := r.deleteRepositorySecret(ctx, proj)
	if err != nil {
		return operatorv1alpha1.ReasonRepositorySecretDeletionFailed, err.Error(), err
	}
	if remaining != nil {
		message := fmt.Sprintf("Waiting for repository Secret %s/%s to be deleted", remaining.Namespace, remaining.Name)
		if len(remaining.Finalizers) > 0 {
			message += fmt.Sprintf(" (blocked by finalizers: %s)", strings.Join(remaining.Finalizers, ", "))
		}
		return operatorv1alpha1.ReasonWaitingForRepositorySecret, message, nil
	}
	return "", "", nil
}

// setGitOpsReady sets the GitOpsReady condition and reports whether it changed.
func setGitOpsReady(proj *operatorv1alpha1.Project, status metav1.ConditionStatus, reason, message string) bool {
	return meta.SetStatusCondition(&proj.Status.Conditions, metav1.Condition{
		Type:               operatorv1alpha1.ConditionTypeGitOpsReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: proj.Generation,
	})
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
			setGitOpsReady(proj, metav1.ConditionFalse, operatorv1alpha1.ReasonGitOpsProjectNotBound, "Referenced Organization not found")
			proj.Status.DisplayName = proj.Name
			if proj.Spec.Name != "" {
				proj.Status.DisplayName = proj.Spec.Name
			}
			if err := r.Status().Update(ctx, proj); err != nil {
				return true, ctrl.Result{}, err
			}
			return true, ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return true, ctrl.Result{}, err
	}

	// Check if Organization is Bound
	boundCond := meta.FindStatusCondition(org.Status.Conditions, operatorv1alpha1.ConditionTypeBound)
	if boundCond == nil || boundCond.Status != metav1.ConditionTrue {
		log.Info("Referenced Organization is not bound to the database", "organization", org.Name)
		meta.SetStatusCondition(&proj.Status.Conditions, metav1.Condition{
			Type:               operatorv1alpha1.ConditionTypeBound,
			Status:             metav1.ConditionFalse,
			Reason:             "OrganizationNotBound",
			Message:            "Referenced Organization is not bound to the database",
			ObservedGeneration: proj.Generation,
		})
		setGitOpsReady(proj, metav1.ConditionFalse, operatorv1alpha1.ReasonGitOpsProjectNotBound, "Referenced Organization is not bound to the database")
		proj.Status.DisplayName = proj.Name
		if proj.Spec.Name != "" {
			proj.Status.DisplayName = proj.Spec.Name
		}
		if err := r.Status().Update(ctx, proj); err != nil {
			return true, ctrl.Result{}, err
		}
		return true, ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return false, ctrl.Result{}, nil
}

func (r *Reconciler) reconcileAvailabilityZones(ctx context.Context, proj *operatorv1alpha1.Project) error {
	var availableZones []operatorv1alpha1.AvailableZoneStatus
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
			found := false
			for _, az := range availableZones {
				if az.Name == azName {
					found = true
					break
				}
			}
			if !found {
				availableZones = append(availableZones, operatorv1alpha1.AvailableZoneStatus{
					Name:        azName,
					ClusterName: cluster.Name,
				})
			}
		} else {
			invalidZones = append(invalidZones, azRef.Name)
		}
	}

	proj.Status.AvailableZones = availableZones
	proj.Status.AvailableAZsCount = len(availableZones)

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
		if err == nil {
			if found {
				status = metav1.ConditionTrue
				reason = operatorv1alpha1.ReasonBoundFound
				message = "Project is bound to the Superphenix Database"
			}
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

// reconcileGitOps ensures the project namespace, repository Secret and ArgoCD Application,
// recording the outcome in the GitOpsReady condition. The caller persists the status.
func (r *Reconciler) reconcileGitOps(ctx context.Context, proj *operatorv1alpha1.Project) error {
	projectSPXID := spxId.ToSPXID(proj.Spec.ProjectID)
	namespaceName := projectSPXID
	appName := fmt.Sprintf("gitops-%s", projectSPXID)

	fail := func(reason string, err error) error {
		setGitOpsReady(proj, metav1.ConditionFalse, reason, err.Error())
		return err
	}

	// Fetch referenced Organization to get names and IDs
	org, err := r.fetchOrganizationForProject(ctx, proj)
	if err != nil {
		return fail(operatorv1alpha1.ReasonOrganizationFetchFailed, err)
	}

	organizationName := org.Spec.Name
	if organizationName == "" {
		organizationName = org.Name
	}

	projectName := proj.Spec.Name
	if projectName == "" {
		projectName = proj.Name
	}

	// Ensure Namespace exists
	if err := r.ensureNamespace(ctx, namespaceName, org.Spec.OrganizationID, proj.Spec.ProjectID); err != nil {
		return fail(operatorv1alpha1.ReasonNamespaceReconcileFailed, err)
	}

	// The repository Secret must exist before the Application references it.
	valuesLocation := getValuesLocation(proj)
	if valuesLocation != nil {
		if err := r.ensureRepositorySecret(ctx, proj, valuesLocation); err != nil {
			return fail(operatorv1alpha1.ReasonRepositorySecretReconcileFailed, err)
		}
	}

	// Determine GitOps parameters
	repoURL, path, targetRevision := r.resolveManifestLocation(proj)

	// Build Helm values
	helmValues, err := r.buildHelmValues(proj, org, organizationName, projectName)
	if err != nil {
		return fail(operatorv1alpha1.ReasonHelmValuesFailed, fmt.Errorf("failed to build helm values: %w", err))
	}

	// Ensure ArgoCD Application exists
	if err := r.ensureArgoApplication(ctx, proj, namespaceName, appName, repoURL, path, targetRevision, helmValues); err != nil {
		return fail(operatorv1alpha1.ReasonApplicationReconcileFailed, err)
	}

	message := fmt.Sprintf("ArgoCD Application %s/%s is configured", namespaceName, appName)
	if valuesLocation != nil {
		message += fmt.Sprintf(" with values repository Secret %s/%s", proj.Namespace, repositorySecretName(proj))
	} else if _, err := r.deleteRepositorySecret(ctx, proj); err != nil {
		// The values repository was removed: the Application no longer references it, drop its credentials.
		return fail(operatorv1alpha1.ReasonRepositorySecretReconcileFailed, err)
	}
	setGitOpsReady(proj, metav1.ConditionTrue, operatorv1alpha1.ReasonGitOpsReconciled, message)
	return nil
}

// getValuesLocation returns the project's values repository, or nil if none is configured.
func getValuesLocation(proj *operatorv1alpha1.Project) *operatorv1alpha1.ProjectValuesLocationSpec {
	if proj.Spec.GitOps == nil || proj.Spec.GitOps.ValuesLocation == nil || proj.Spec.GitOps.ValuesLocation.RepoURL == "" {
		return nil
	}
	return proj.Spec.GitOps.ValuesLocation
}

var errRepositorySecretTerminating = errors.New("repository Secret is being deleted")

func repositorySecretName(proj *operatorv1alpha1.Project) string {
	return fmt.Sprintf("repository-%s", proj.Spec.ProjectID)
}

func (r *Reconciler) ensureRepositorySecret(ctx context.Context, proj *operatorv1alpha1.Project, valuesLocation *operatorv1alpha1.ProjectValuesLocationSpec) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      repositorySecretName(proj),
			Namespace: proj.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
		if !secret.DeletionTimestamp.IsZero() {
			return errRepositorySecretTerminating
		}
		if secret.Labels == nil {
			secret.Labels = make(map[string]string)
		}
		secret.Labels[repositorySecretTypeLabel] = "repository"
		controllerutil.AddFinalizer(secret, FinalizerName)
		if err := controllerutil.SetControllerReference(proj, secret, r.Scheme); err != nil {
			return err
		}

		secret.Type = corev1.SecretTypeOpaque
		secret.Data = map[string][]byte{
			"name": []byte(secret.Name),
			"type": []byte("git"),
			"url":  []byte(valuesLocation.RepoURL),
		}
		if credentials := valuesLocation.Credentials; credentials != nil {
			if credentials.Username != "" {
				secret.Data["username"] = []byte(credentials.Username)
			}
			if credentials.Password != "" {
				secret.Data["password"] = []byte(credentials.Password)
			}
			if credentials.Insecure {
				secret.Data["insecure"] = []byte("true")
			}
			if credentials.ForceHttpBasicAuth {
				secret.Data["forceHttpBasicAuth"] = []byte("true")
			}
			if credentials.EnableLfs {
				secret.Data["enableLfs"] = []byte("true")
			}
			if credentials.SshPrivateKey != "" {
				secret.Data["sshPrivateKey"] = []byte(credentials.SshPrivateKey)
			}
		}
		return nil
	})
	if errors.Is(err, errRepositorySecretTerminating) {
		// Deleted out of band while the project is alive: release it so the
		// returned error requeues and the next reconcile recreates it.
		if _, delErr := r.deleteRepositorySecret(ctx, proj); delErr != nil {
			return delErr
		}
	}
	if err != nil {
		return fmt.Errorf("failed to reconcile repository secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}
	return nil
}

// deleteRepositorySecret releases and deletes the project's repository Secret.
// It returns nil once the Secret is confirmed gone, otherwise the Secret as last seen
// (gone-ness is confirmed on the next pass).
func (r *Reconciler) deleteRepositorySecret(ctx context.Context, proj *operatorv1alpha1.Project) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{Name: repositorySecretName(proj), Namespace: proj.Namespace}
	if err := r.Get(ctx, key, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return secret, fmt.Errorf("failed to get repository Secret %s: %w", key, err)
	}

	if controllerutil.RemoveFinalizer(secret, FinalizerName) {
		if err := r.Update(ctx, secret); client.IgnoreNotFound(err) != nil {
			return secret, fmt.Errorf("failed to release repository Secret %s: %w", key, err)
		}
	}
	if err := r.Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
		return secret, fmt.Errorf("failed to delete repository Secret %s: %w", key, err)
	}
	return secret, nil
}

func (r *Reconciler) fetchOrganizationForProject(ctx context.Context, proj *operatorv1alpha1.Project) (*operatorv1alpha1.Organization, error) {
	orgNamespace := proj.Spec.OrganizationRef.Namespace
	if orgNamespace == "" {
		orgNamespace = proj.Namespace
	}
	org := &operatorv1alpha1.Organization{}
	if err := r.Get(ctx, types.NamespacedName{Name: proj.Spec.OrganizationRef.Name, Namespace: orgNamespace}, org); err != nil {
		return nil, fmt.Errorf("failed to fetch organization %s: %w", proj.Spec.OrganizationRef.Name, err)
	}
	return org, nil
}

func (r *Reconciler) ensureNamespace(ctx context.Context, namespaceName, organizationID, projectID string) error {
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
		ns.Labels[spxId.SpxLabelOrganizationID] = spxId.ToSPXID(organizationID)
		ns.Labels[spxId.SpxLabelProjectID] = spxId.ToSPXID(projectID)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile namespace %s: %w", namespaceName, err)
	}
	return nil
}

func (r *Reconciler) resolveManifestLocation(proj *operatorv1alpha1.Project) (string, string, string) {
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
	return repoURL, path, targetRevision
}

func (r *Reconciler) buildHelmValues(proj *operatorv1alpha1.Project, org *operatorv1alpha1.Organization, organizationName, projectName string) (string, error) {
	type azValue struct {
		Name    string `yaml:"name"`
		Cluster string `yaml:"cluster"`
	}

	type projectValues struct {
		ProjectID         string    `yaml:"projectID"`
		Name              string    `yaml:"name"`
		AvailabilityZones []azValue `yaml:"availabilityZones"`
	}

	type organizationValues struct {
		OrganizationID string `yaml:"organizationID"`
		Name           string `yaml:"name"`
	}

	type helmValues struct {
		Organization organizationValues `yaml:"organization"`
		Project      projectValues      `yaml:"project"`
	}

	// Collect AZ details
	var azs []azValue
	for _, az := range proj.Status.AvailableZones {
		azs = append(azs, azValue{
			Name:    az.Name,
			Cluster: az.ClusterName,
		})
	}

	values := helmValues{
		Organization: organizationValues{
			OrganizationID: org.Spec.OrganizationID,
			Name:           organizationName,
		},
		Project: projectValues{
			ProjectID:         proj.Spec.ProjectID,
			Name:              projectName,
			AvailabilityZones: azs,
		},
	}

	data, err := yaml.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *Reconciler) ensureArgoApplication(ctx context.Context, proj *operatorv1alpha1.Project, namespaceName, appName, repoURL, path, targetRevision, helmValues string) error {
	app := &argov1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespaceName,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, app, func() error {
		if app.Labels == nil {
			app.Labels = make(map[string]string)
		}
		app.Labels["operator.superphenix.net/managed"] = "true"

		controllerutil.AddFinalizer(app, argocdResourcesFinalizer)

		source := argov1alpha1.ApplicationSource{
			RepoURL:        repoURL,
			Path:           path,
			TargetRevision: targetRevision,
			Helm: &argov1alpha1.ApplicationSourceHelm{
				Values: helmValues,
			},
		}

		valuesLocation := getValuesLocation(proj)
		if valuesLocation != nil {
			source.Helm.ValueFiles = []string{"$values/**/*.yaml", "$values/**/*.yml"}
			source.Helm.IgnoreMissingValueFiles = true

			valuesTargetRevision := valuesLocation.TargetRevision
			if valuesTargetRevision == "" {
				valuesTargetRevision = "HEAD"
			}

			app.Spec.Source = nil
			app.Spec.Sources = []argov1alpha1.ApplicationSource{
				source,
				{
					RepoURL:        valuesLocation.RepoURL,
					TargetRevision: valuesTargetRevision,
					Ref:            "values",
				},
			}
		} else {
			app.Spec.Source = &source
			app.Spec.Sources = nil
		}

		app.Spec.Destination = argov1alpha1.ApplicationDestination{
			Name:      "in-cluster",
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
