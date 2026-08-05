package management

import (
	"context"
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

	// Failures here are non-fatal: ArgoCD may already be running or managed by other means.
	if err := r.ensureInitialHelmInstall(ctx); err != nil {
		log.Error(err, "Initial Helm install failed, continuing")
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

// isHelmReleaseInstalled reports whether a Helm release with the given name is already deployed.
func isHelmReleaseInstalled(actionConfig *action.Configuration, releaseName string) bool {
	hist := action.NewHistory(actionConfig)
	hist.Max = 1
	_, err := hist.Run(releaseName)
	return err == nil
}

// locateAndLoadArgoCDChart resolves, downloads, and loads the argo-cd Helm chart into memory.
func locateAndLoadArgoCDChart(clientInstall *action.Install, helmSettings *cli.EnvSettings) (*chart.Chart, error) {
	cp, err := clientInstall.ChartPathOptions.LocateChart("argo-cd", helmSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to locate ArgoCD chart: %w", err)
	}

	ch, err := loader.Load(cp)
	if err != nil {
		return nil, fmt.Errorf("failed to load ArgoCD chart: %w", err)
	}

	return ch, nil
}

// runHelmInstall executes the Helm installation for the given chart.
// Namespace-not-found errors are silenced because they are a known envtest limitation.
func runHelmInstall(ctx context.Context, clientInstall *action.Install, ch *chart.Chart) error {
	log := logf.FromContext(ctx)

	if _, err := clientInstall.Run(ch, nil); err != nil {
		if strings.Contains(err.Error(), "namespaces") && strings.Contains(err.Error(), "not found") {
			log.Info("Namespace not found during Helm install; treating as envtest limitation and skipping")
			return nil
		}
		return fmt.Errorf("failed to install initial ArgoCD Helm chart: %w", err)
	}

	return nil
}

// ensureInitialHelmInstall performs a one-time default Helm installation of ArgoCD.
// This is idempotent: it is a no-op when the release already exists.
func (r *Reconciler) ensureInitialHelmInstall(ctx context.Context) error {
	log := logf.FromContext(ctx)

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

	if isHelmReleaseInstalled(actionConfig, ArgoCDApp) {
		log.Info("ArgoCD Helm release already exists, skipping initial install (delete the helm release secret to override)")
		return nil
	}

	log.Info("Performing initial ArgoCD Helm install", "chart", "argo-cd", "version", r.ArgoCDChartVersion)

	clientInstall := action.NewInstall(actionConfig)
	clientInstall.ReleaseName = ArgoCDApp
	clientInstall.Namespace = r.OperatorNamespace
	clientInstall.RepoURL = r.ArgoCDChartURL
	clientInstall.Version = r.ArgoCDChartVersion
	clientInstall.Wait = false
	clientInstall.CreateNamespace = false

	ch, err := locateAndLoadArgoCDChart(clientInstall, helmSettings)
	if err != nil {
		return err
	}

	return runHelmInstall(ctx, clientInstall, ch)
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
