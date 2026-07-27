package config

import (
	"os"

	"github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/typed/application/v1alpha1"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	ArgoClient v1alpha1.ArgoprojV1alpha1Interface

	// K8sClient is a standard Kubernetes client to interact with a cluster
	K8sClient *kubernetes.Clientset
)

func InitK8SConfig() {

	// defaultClientConfig() prepares config using kubeconfig.
	// typically, you need to set env variable, KUBECONFIG=<path-to-kubeconfig>/.kubeconfig
	clientConfig := defaultClientConfig(&pflag.FlagSet{})

	config, err := clientConfig.ClientConfig()
	if err != nil {
		log.Fatal().AnErr("cannot obtain K8S Rest Config: %v\n", err)
	}

	k8sConfig, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal().AnErr("cannot obtain K8S ClientSet: %v\n", err)
	}
	K8sClient = k8sConfig

	argoClient, err := v1alpha1.NewForConfig(config)
	if err != nil {
		log.Fatal().AnErr("cannot obtain Argo Client: %v\n", err)
	}
	ArgoClient = argoClient

}

// defaultClientConfig is a copy of the kubecli function to fetch the client config
func defaultClientConfig(flags *pflag.FlagSet) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	flags.StringVar(&loadingRules.ExplicitPath, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}

	flagNames := clientcmd.RecommendedConfigOverrideFlags("")
	// short flagnames are disabled by default.  These are here for compatibility with existing scripts
	flagNames.ClusterOverrideFlags.APIServer.ShortName = "s"

	clientcmd.BindOverrideFlags(overrides, flags, flagNames)
	clientConfig := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, overrides, os.Stdin)

	return clientConfig
}
