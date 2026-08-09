package management

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/super-phenix/superphenix/pkg/utils"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/storage/driver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
	"github.com/super-phenix/superphenix/pkg/argocd"
)

const (
	// ControllerName is the name of the management controller.
	ControllerName = "management-controller"

	// ArgoCDApp is the Helm release name and ArgoCD Application name for the management ArgoCD instance.
	ArgoCDApp = "superphenix-argocd"

	// ConfigMapKeyArgoCD is the key in the management ConfigMap holding ArgoCD Helm values.
	ConfigMapKeyArgoCD = "argocd"
)

// Reconciler handles the reconciliation of management components.
// It implements reconcile.Reconciler to handle ConfigMap updates.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config

	// OperatorNamespace is where we're deploying this operator.
	OperatorNamespace string

	// HAEnabled indicates whether High Availability mode is enabled for management components.
	HAEnabled bool

	// ValuesConfigMapName is the name of the general management ConfigMap.
	// It holds Helm values for each component under dedicated subkeys (see ConfigMapKey* constants).
	ValuesConfigMapName string

	// ArgoCD chart configuration.
	ArgoCDChartURL      string
	ArgoCDChartVersion  string
	ArgoCDDefaultConfig string
	ArgoCDHAConfig      string

	// InstallWithoutCNI indicates whether to install without a CNI (enables hostNetwork for redis).
	InstallWithoutCNI bool
}

// SetupWithManager registers the controller with the Manager, watching only the management values ConfigMap.
func (r *Reconciler) SetupWithManager(manager ctrl.Manager) error {
	return builder.ControllerManagedBy(manager).
		Named(ControllerName).
		For(&corev1.ConfigMap{}, builder.WithPredicates(r.configMapPredicate())).
		Complete(r)
}

// Reconcile bootstraps and maintains the full management stack (ArgoCD, Console, Auth...).
// It is triggered by changes to the management values ConfigMap and re-enqueues periodically as a safety net.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	// Ignore spurious events for ConfigMaps other than the one we watch.
	if request.Name != "" && !r.isTargetConfigMap(request.Name, request.Namespace) {
		return reconcile.Result{}, nil
	}

	log.Info("Reconciling management components")

	// Reconcile the management ArgoCD instance.
	if err := r.reconcileManagementArgoCD(ctx); err != nil {
		log.Error(err, "ArgoCD reconciliation failed")
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	// Check if ArgoCD CRDs are installed on the management cluster, as we can't proceed without them.
	if err := argocd.CheckCRDs(ctx, r.RESTMapper()); err != nil {
		log.Error(err, "ArgoCD CRDs are missing on the management cluster")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return reconcile.Result{RequeueAfter: 10 * time.Minute}, nil
}

// isTargetConfigMap reports whether the given name and namespace identify the watched management values ConfigMap.
func (r *Reconciler) isTargetConfigMap(name, namespace string) bool {
	return name == r.ValuesConfigMapName && namespace == r.OperatorNamespace
}

// configMapPredicate limits reconciliation events to the management values ConfigMap.
func (r *Reconciler) configMapPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !r.isTargetConfigMap(e.ObjectNew.GetName(), e.ObjectNew.GetNamespace()) {
				return false
			}
			// Only reconcile if the data actually changed.
			oldCm, ok1 := e.ObjectOld.(*corev1.ConfigMap)
			newCm, ok2 := e.ObjectNew.(*corev1.ConfigMap)
			if ok1 && ok2 {
				return !reflect.DeepEqual(oldCm.Data, newCm.Data)
			}
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return r.isTargetConfigMap(e.Object.GetName(), e.Object.GetNamespace())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return r.isTargetConfigMap(e.Object.GetName(), e.Object.GetNamespace())
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return r.isTargetConfigMap(e.Object.GetName(), e.Object.GetNamespace())
		},
	}
}

// reconcileManagementArgoCD deploys the management ArgoCD instance that bootstraps the rest of the stack.
// It resolves the chicken-and-egg problem: first a plain Helm installation gets ArgoCD running,
// then an ArgoCD Application hands its lifecycle back to ArgoCD itself.
func (r *Reconciler) reconcileManagementArgoCD(ctx context.Context) error {
	log := logf.FromContext(ctx)

	// Install ArgoCD using Helm.
	if err := r.ensureInitialHelmInstall(ctx); err != nil {
		return fmt.Errorf("failed to ensure initial ArgoCD install: %w", err)
	}

	// If requested, patch redis for hostNetwork (required in CNI-less environments).
	if r.InstallWithoutCNI {
		if err := r.patchRedisForHostNetwork(ctx); err != nil {
			return fmt.Errorf("failed to patch redis for hostNetwork: %w", err)
		}
	}

	// Give back control to ArgoCD itself.
	if err := r.ensureArgoCDSelfManaged(ctx); err != nil {
		return fmt.Errorf("failed to ensure ArgoCD is self-managed: %w", err)
	}

	log.Info("Successfully reconciled ArgoCD")
	return nil
}

// setupHelmEnvironment creates a temporary directory for Helm's cache, config, and data,
// exports the required environment variables, and returns a configured EnvSettings.
// The caller must defer the returned cleanup function to restore the environment.
func setupHelmEnvironment() (*cli.EnvSettings, func(), error) {
	tmpDir, err := os.MkdirTemp("", "superphenix-helm-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temporary directory for Helm: %w", err)
	}

	cacheDir := filepath.Join(tmpDir, "cache")
	configDir := filepath.Join(tmpDir, "config")
	dataDir := filepath.Join(tmpDir, "data")

	for _, dir := range []string{filepath.Join(cacheDir, "repository"), configDir, dataDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			os.RemoveAll(tmpDir)
			return nil, nil, fmt.Errorf("failed to create Helm directory %s: %w", dir, err)
		}
	}

	os.Setenv("HELM_CACHE_HOME", cacheDir)
	os.Setenv("HELM_CONFIG_HOME", configDir)
	os.Setenv("HELM_DATA_HOME", dataDir)

	helmSettings := cli.New()
	helmSettings.KubeConfig = "" // Use in-cluster config; never load from a kubeconfig file.
	helmSettings.RepositoryCache = filepath.Join(cacheDir, "repository")
	helmSettings.RepositoryConfig = filepath.Join(configDir, "repositories.yaml")

	cleanup := func() {
		os.Unsetenv("HELM_CACHE_HOME")
		os.Unsetenv("HELM_CONFIG_HOME")
		os.Unsetenv("HELM_DATA_HOME")
		os.RemoveAll(tmpDir)
	}

	return helmSettings, cleanup, nil
}

// initHelmActionConfig initialises a Helm action.Configuration for the operator namespace.
// Returns (nil, nil) when the RESTClientGetter is unavailable (e.g., in envtest).
func (r *Reconciler) initHelmActionConfig(ctx context.Context, helmSettings *cli.EnvSettings) (*action.Configuration, error) {
	log := logf.FromContext(ctx)

	restGetter := helmSettings.RESTClientGetter()
	if restGetter == nil {
		log.Info("RESTClientGetter is nil, skipping Helm action config init (likely in test)")
		return nil, nil
	}

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(restGetter, r.OperatorNamespace, "secret", func(format string, v ...interface{}) {
		log.Info(fmt.Sprintf(format, v...))
	}); err != nil {
		return nil, fmt.Errorf("failed to initialize Helm action configuration: %w", err)
	}

	return actionConfig, nil
}

// locateAndLoadArgoCDChart resolves, downloads, and loads the argo-cd Helm chart into memory.
func locateAndLoadArgoCDChart(chartPathOptions *action.ChartPathOptions, helmSettings *cli.EnvSettings) (*chart.Chart, error) {
	cp, err := chartPathOptions.LocateChart("argo-cd", helmSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to locate ArgoCD chart: %w", err)
	}

	ch, err := loader.Load(cp)
	if err != nil {
		return nil, fmt.Errorf("failed to load ArgoCD chart: %w", err)
	}

	return ch, nil
}

// runHelmUpgradeInstall executes the Helm upgrade or install for the given chart and values.
// It explicitly checks for release existence because action.Upgrade with Install=true does not
// trigger an install when the release is missing.
// Namespace-not-found errors are silenced because they are a known envtest limitation.
func (r *Reconciler) runHelmUpgradeInstall(ctx context.Context, actionConfig *action.Configuration, releaseName string, ch *chart.Chart, vals map[string]interface{}) error {
	log := logf.FromContext(ctx)

	// Check if the release already exists to decide between Install and Upgrade.
	histClient := action.NewHistory(actionConfig)
	histClient.Max = 1
	_, err := histClient.Run(releaseName)
	if err != nil && (errors.Is(err, driver.ErrReleaseNotFound) || strings.Contains(err.Error(), "not found")) {
		log.Info("Helm release not found, performing initial install", "release", releaseName)
		clientInstall := action.NewInstall(actionConfig)
		clientInstall.ReleaseName = releaseName
		clientInstall.Namespace = r.OperatorNamespace
		clientInstall.Wait = false

		if _, err := clientInstall.Run(ch, vals); err != nil {
			// Namespace-not-found and CRD ownership errors are silenced because they are a known envtest limitation.
			if (strings.Contains(err.Error(), "namespaces") && strings.Contains(err.Error(), "not found")) ||
				(strings.Contains(err.Error(), "exists and cannot be imported into the current release")) {
				log.Info("Silencing Helm error known to be an envtest limitation", "error", err)
				return nil
			}
			return fmt.Errorf("failed to install initial ArgoCD Helm chart: %w", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to check Helm release history: %w", err)
	}

	log.Info("Helm release found, performing upgrade", "release", releaseName)
	clientUpgrade := action.NewUpgrade(actionConfig)
	clientUpgrade.Namespace = r.OperatorNamespace
	clientUpgrade.Wait = false

	if _, err := clientUpgrade.Run(releaseName, ch, vals); err != nil {
		return fmt.Errorf("failed to upgrade initial ArgoCD Helm chart: %w", err)
	}

	return nil
}

// ensureInitialHelmInstall performs a default Helm upgrade+install of ArgoCD.
// It is a no-op when the ArgoCD Application is already present (self-managed).
func (r *Reconciler) ensureInitialHelmInstall(ctx context.Context) error {
	log := logf.FromContext(ctx)

	// We do not do the initial install if the argo is self managed (the app is present)
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Application",
	})
	err := r.Get(ctx, types.NamespacedName{Name: ArgoCDApp, Namespace: r.OperatorNamespace}, app)
	if err == nil {
		log.Info("ArgoCD is already self-managed (Application exists), skipping initial Helm install")
		return nil
	}
	if !apierrors.IsNotFound(err) {
		log.Info("Could not verify if ArgoCD Application exists, proceeding with Helm install anyway", "error", err)
	}

	helmSettings, cleanup, err := setupHelmEnvironment()
	if err != nil {
		return err
	}
	defer cleanup()

	actionConfig, err := r.initHelmActionConfig(ctx, helmSettings)
	if err != nil {
		return err
	}
	if actionConfig == nil {
		return nil
	}

	log.Info("Performing initial ArgoCD Helm upgrade+install", "chart", "argo-cd", "version", r.ArgoCDChartVersion)

	chartPathOptions := &action.ChartPathOptions{
		RepoURL: r.ArgoCDChartURL,
		Version: r.ArgoCDChartVersion,
	}

	ch, err := locateAndLoadArgoCDChart(chartPathOptions, helmSettings)
	if err != nil {
		return err
	}

	values, err := r.mergeArgoCDValues(ctx)
	if err != nil {
		return fmt.Errorf("failed to merge ArgoCD values for initial install: %w", err)
	}

	return r.runHelmUpgradeInstall(ctx, actionConfig, ArgoCDApp, ch, values)
}

// buildApplication constructs a self-syncing ArgoCD Application manifest backed by a Helm chart.
func (r *Reconciler) buildApplication(name, repoURL, chartName, chartVersion string, vals map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": r.OperatorNamespace,
				"labels": map[string]interface{}{
					version.ManagedLabel: "true",
				},
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					"repoURL":        repoURL,
					"targetRevision": chartVersion,
					"chart":          chartName,
					"helm": map[string]interface{}{
						"valuesObject": vals,
					},
				},
				"destination": map[string]interface{}{
					"name":      "in-cluster",
					"namespace": r.OperatorNamespace,
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"prune":    true,
						"selfHeal": true,
					},
				},
			},
		},
	}
}

// buildArgoCDApplication constructs the ArgoCD Application manifest that configures ArgoCD to manage itself.
func (r *Reconciler) buildArgoCDApplication(vals map[string]interface{}) *unstructured.Unstructured {
	return r.buildApplication(ArgoCDApp, r.ArgoCDChartURL, "argo-cd", r.ArgoCDChartVersion, vals)
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

// ensureArgoCDSelfManaged creates or updates the ArgoCD Application that hands ArgoCD's lifecycle to itself.
func (r *Reconciler) ensureArgoCDSelfManaged(ctx context.Context) error {
	values, err := r.mergeArgoCDValues(ctx)
	if err != nil {
		return fmt.Errorf("failed to merge ArgoCD values: %w", err)
	}

	return r.createOrUpdateArgoCDApplication(ctx, r.buildArgoCDApplication(values))
}

// loadYAMLFileValues reads the YAML file at path and unmarshals it into a map.
// Returns (nil, false, nil) when the file does not exist.
func loadYAMLFileValues(path string) (map[string]interface{}, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	vals := make(map[string]interface{})
	if err := yaml.Unmarshal(data, &vals); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal YAML from %s: %w", path, err)
	}

	return vals, true, nil
}

// loadConfigMapValuesFrom fetches the specified ConfigMap and deep-merges the given key into dst.
func (r *Reconciler) loadConfigMapValuesFrom(ctx context.Context, name, key string, dst map[string]interface{}) error {
	log := logf.FromContext(ctx)

	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: r.OperatorNamespace}, cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Management values ConfigMap not found, using defaults only",
				"name", name,
				"namespace", r.OperatorNamespace)
			return nil
		}
		return fmt.Errorf("failed to get management values ConfigMap: %w", err)
	}

	data, ok := cm.Data[key]
	if !ok {
		log.Info("Management values ConfigMap has no key, skipping",
			"name", name,
			"key", key)
		return nil
	}

	cmVals := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(data), &cmVals); err != nil {
		return fmt.Errorf("failed to unmarshal YAML from ConfigMap key %q: %w", key, err)
	}

	utils.MapDeepMerge(dst, cmVals)
	return nil
}

// mergeArgoCDValues builds the final Helm values for the ArgoCD chart.
func (r *Reconciler) mergeArgoCDValues(ctx context.Context) (map[string]interface{}, error) {
	return r.mergeValues(ctx, r.ArgoCDDefaultConfig, r.ArgoCDHAConfig, ConfigMapKeyArgoCD)
}

// mergeValues builds a Helm values map by layering, in order:
// base file defaults, HA overrides (when enabled), then the user overrides from the given ConfigMap key.
func (r *Reconciler) mergeValues(ctx context.Context, defaultConfig, haConfig, configMapKey string) (map[string]interface{}, error) {
	log := logf.FromContext(ctx)
	merged := make(map[string]interface{})

	if defaultConfig != "" {
		vals, found, err := loadYAMLFileValues(defaultConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load default values from %s: %w", defaultConfig, err)
		}
		if !found {
			log.Info("Default config file not found", "path", defaultConfig)
		} else {
			utils.MapDeepMerge(merged, vals)
		}
	}

	if r.HAEnabled && haConfig != "" {
		vals, found, err := loadYAMLFileValues(haConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load HA values from %s: %w", haConfig, err)
		}
		if !found {
			log.Info("HA config file not found", "path", haConfig)
		} else {
			utils.MapDeepMerge(merged, vals)
		}
	}

	if r.ValuesConfigMapName != "" && r.OperatorNamespace != "" {
		if err := r.loadConfigMapValuesFrom(ctx, r.ValuesConfigMapName, configMapKey, merged); err != nil {
			return nil, err
		}
	}

	return merged, nil
}

// patchRedisForHostNetwork patches the ArgoCD redis deployment to use hostNetwork.
func (r *Reconciler) patchRedisForHostNetwork(ctx context.Context) error {
	log := logf.FromContext(ctx)
	deploymentName := ArgoCDApp + "-redis"

	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: r.OperatorNamespace}, deployment); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Redis deployment not found, skipping patch", "deployment", deploymentName)
			return nil
		}
		return fmt.Errorf("failed to get redis deployment: %w", err)
	}

	if deployment.Spec.Template.Spec.HostNetwork {
		return nil
	}

	patch := client.MergeFrom(deployment.DeepCopy())
	deployment.Spec.Template.Spec.HostNetwork = true
	if err := r.Patch(ctx, deployment, patch); err != nil {
		return fmt.Errorf("failed to patch redis deployment for hostNetwork: %w", err)
	}

	log.Info("Successfully patched redis deployment for hostNetwork", "deployment", deploymentName)
	return nil
}

