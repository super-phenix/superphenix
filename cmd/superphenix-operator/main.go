package main

import (
	"context"
	"crypto/tls"
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	operatorv1alpha1 "github.com/super-phenix/superphenix/api/operator/v1alpha1"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/cluster"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/management"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/telemetry"
	"github.com/super-phenix/superphenix/internal/superphenix-operator/version"
	"github.com/super-phenix/superphenix/pkg/argocd"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var argocdChartURL string
	var argocdChartVersion string
	var valuesConfigMapName string
	var clustersConfigMapName string
	var operatorNamespace string
	var tlsOpts []func(*tls.Config)
	var argocdDefaultConfig string
	var argocdHAConfig string
	var haEnabled bool
	var systemChartURL string
	var systemChartName string
	var systemChartVersion string
	var syncPeriod time.Duration
	var syncTimeout time.Duration
	var talosManagerChartURL string
	var talosManagerChartVersion string
	var disableTelemetry bool
	var telemetryEndpoint string
	var disableVersionValidation bool
	var installWithoutCNI bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&argocdChartURL, "argocd-chart-url", "https://argoproj.github.io/argo-helm", "The URL of the ArgoCD chart repository")
	flag.StringVar(&argocdChartVersion, "argocd-chart-version", "9.7.0", "The version of the ArgoCD chart")
	flag.StringVar(&valuesConfigMapName, "values-configmap-name", "superphenix-mgmt-values", "The name of the general management ConfigMap holding Helm values for each component under dedicated sub-keys")
	flag.StringVar(&clustersConfigMapName, "clusters-configmap-name", "superphenix-clusters-config", "The name of the ConfigMap where all clusters will append their configuration")
	flag.StringVar(&argocdDefaultConfig, "argocd-default-config", "/etc/superphenix/argocd/default/values.yaml", "Path to the default ArgoCD configuration file")
	flag.StringVar(&argocdHAConfig, "argocd-ha-config", "/etc/superphenix/argocd/ha/values.yaml", "Path to the HA ArgoCD configuration file")
	flag.BoolVar(&haEnabled, "ha-enabled", false, "Whether to enable HA for ArgoCD")
	flag.StringVar(&systemChartURL, "system-chart-url", "oci://ghcr.io/super-phenix/charts", "The URL of the Superphenix system chart repository")
	flag.StringVar(&systemChartName, "system-chart-name", "superphenix-system", "The name of the Superphenix system chart")
	flag.StringVar(&systemChartVersion, "system-chart-version", "0.0.0", "The version of the Superphenix system chart")
	flag.DurationVar(&syncPeriod, "sync-period", 5*time.Minute, "The interval at which to periodically resync sub-applications")
	flag.DurationVar(&syncTimeout, "sync-timeout", 15*time.Minute, "The duration after which an in-progress sub-application sync is considered stuck, aborted, and restarted")
	flag.StringVar(&talosManagerChartURL, "talos-manager-chart-url", "ghcr.io/super-phenix/charts", "The repository URL for the talos-manager chart")
	flag.StringVar(&talosManagerChartVersion, "talos-manager-chart-version", "0.1.0", "The version for the talos-manager chart")
	flag.StringVar(&operatorNamespace, "operator-namespace", os.Getenv("OPERATOR_NAMESPACE"), "The namespace where the operator is deployed")
	flag.BoolVar(&disableTelemetry, "disable-telemetry", false, "Disable sending anonymous telemetry to the Superphenix open-source project")
	flag.StringVar(&telemetryEndpoint, "telemetry-endpoint", telemetry.DefaultEndpoint, "URL of the telemetry ingest endpoint")
	flag.BoolVar(&disableVersionValidation, "disable-version-validation", false, "Disable validation of versions entirely")
	flag.BoolVar(&installWithoutCNI, "install-without-cni", false, "Whether to install components without CNI (enables hostNetwork for redis)")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("Disabling HTTP/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		webhookServerOptions.CertDir = webhookCertPath
		webhookServerOptions.CertName = webhookCertName
		webhookServerOptions.KeyName = webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	// Restrict the ArgoCD Application informer to Applications the operator manages:
	// root cluster Apps, superphenix-system child Apps, and management stack Apps. All three
	// cohorts carry the operator.superphenix.net/managed=true label and live in the
	// operator's namespace. Without this filter the informer LIST/WATCHes every
	// Application in the cluster.
	argoApp := &unstructured.Unstructured{}
	argoApp.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Application",
	})
	managedSel, err := labels.Parse("operator.superphenix.net/managed=true")
	if err != nil {
		setupLog.Error(err, "Failed to parse managed label selector")
		os.Exit(1)
	}
	argoAppByObject := cache.ByObject{Label: managedSel}
	if operatorNamespace != "" {
		argoAppByObject.Namespaces = map[string]cache.Config{operatorNamespace: {}}
	}

	// On a brand new cluster the ArgoCD CRDs are not yet installed, so we cannot
	// register a ByObject filter for Applications (controller-runtime would fail
	// to look up the type's REST scope during manager construction). Skip the
	// filter for now; a background goroutine polls for the CRDs and exits the
	// process when they appear so Kubernetes restarts us with the filter applied.
	cfg := ctrl.GetConfigOrDie()
	cacheOpts := cache.Options{}
	if argoCDCRDsPresent(cfg) {
		cacheOpts.ByObject = map[client.Object]cache.ByObject{
			argoApp: argoAppByObject,
		}
	} else {
		setupLog.Info("ArgoCD CRDs not yet installed; starting without the ArgoCD Application cache filter. The operator will restart once the CRDs are available.")
		go waitForArgoCDCRDsThenExit(cfg)
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                        scheme,
		Metrics:                       metricsServerOptions,
		WebhookServer:                 webhookServer,
		HealthProbeBindAddress:        probeAddr,
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              "d19ca498.superphenix.net",
		Cache:                         cacheOpts,
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}

	if err := (&cluster.Reconciler{
		Client:                   mgr.GetClient(),
		Scheme:                   mgr.GetScheme(),
		OperatorNamespace:        operatorNamespace,
		ClustersConfigMapName:    clustersConfigMapName,
		SystemChartURL:           systemChartURL,
		SystemChartName:          systemChartName,
		SystemChartVersion:       systemChartVersion,
		SyncPeriod:               syncPeriod,
		SyncTimeout:              syncTimeout,
		TalosManagerChartURL:     talosManagerChartURL,
		TalosManagerChartVersion: talosManagerChartVersion,
		DisableVersionValidation: disableVersionValidation,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "Cluster")
		os.Exit(1)
	}

	setupLog.Info("Setting up management components reconciler")
	if err := (&management.Reconciler{
		Client:              mgr.GetClient(),
		Scheme:              mgr.GetScheme(),
		Config:              mgr.GetConfig(),
		OperatorNamespace:   operatorNamespace,
		ValuesConfigMapName: valuesConfigMapName,
		HAEnabled:           haEnabled,
		ArgoCDChartURL:      argocdChartURL,
		ArgoCDChartVersion:  argocdChartVersion,
		ArgoCDDefaultConfig: argocdDefaultConfig,
		ArgoCDHAConfig:      argocdHAConfig,
		InstallWithoutCNI:   installWithoutCNI,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create management controller")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if !disableTelemetry {
		runner := &telemetry.Runner{
			Collector: &telemetry.Collector{
				Client:          mgr.GetClient(),
				OperatorVersion: version.OperatorVersion,
				Namespace:       operatorNamespace,
				SystemVersion:   systemChartVersion,
				ArgoCDVersion:   argocdChartVersion,
			},
			Client: telemetry.NewClient(telemetryEndpoint),
		}

		// Register the telemetry runner. Since it implements LeaderElectionRunnable,
		// it will only start when the manager is elected leader.
		if err := mgr.Add(runner); err != nil {
			setupLog.Error(err, "Failed to register telemetry runner")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Failed to run manager")
		os.Exit(1)
	}
}

// argoCDCRDsPresent reports whether the ArgoCD Application and AppProject CRDs
// are already registered on the API server.
func argoCDCRDsPresent(cfg *rest.Config) bool {
	mapper, err := newRESTMapper(cfg)
	if err != nil {
		setupLog.Error(err, "Failed to build REST mapper for ArgoCD CRD check")
		return false
	}
	return argocd.CheckCRDs(context.Background(), mapper) == nil
}

// waitForArgoCDCRDsThenExit polls until the ArgoCD CRDs become available,
// then exits so that Kubernetes restarts the pod with the ArgoCD Application
// cache filter properly applied.
func waitForArgoCDCRDsThenExit(cfg *rest.Config) {
	const pollInterval = 30 * time.Second
	for {
		time.Sleep(pollInterval)
		mapper, err := newRESTMapper(cfg)
		if err != nil {
			continue
		}
		if err := argocd.CheckCRDs(context.Background(), mapper); err == nil {
			setupLog.Info("ArgoCD CRDs are now available, exiting so the pod restarts with the Application cache filter applied")
			os.Exit(0)
		}
	}
}

// newRESTMapper builds a fresh REST mapper against the API server. A new mapper
// is created for each check so cached negative lookups do not mask CRDs that
// were installed after the previous attempt.
func newRESTMapper(cfg *rest.Config) (meta.RESTMapper, error) {
	httpClient, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, err
	}
	return apiutil.NewDynamicRESTMapper(cfg, httpClient)
}
