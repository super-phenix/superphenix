package config

import (
	clientset "github.com/kubeovn/kube-ovn/pkg/client/clientset/versioned"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	snapshot "kubevirt.io/client-go/externalsnapshotter/typed/volumesnapshot/v1"
	"kubevirt.io/client-go/kubecli"
	"sigs.k8s.io/cluster-api/api/core/v1beta2"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = v1beta2.AddToScheme(scheme)
}

var (
	// K8sClient is a standard Kubernetes client to interact with a cluster
	K8sClient kubernetes.Interface
	// VirtClient is a client to interact with Kubevirt on a cluster
	VirtClient kubecli.KubevirtClient
	// KubeOvnClient is a client to interact with kubeovn on a cluster
	KubeOvnClient clientset.Interface
	// SnapshotClient is a client to interact with snapshot on a cluster
	SnapshotClient snapshot.SnapshotV1Interface

	InformerClient *dynamic.DynamicClient

	DynamicClientSet dynamic.Interface
)

func InitK8SConfig() {

	// kubecli.DefaultClientConfig() prepares config using kubeconfig.
	// typically, you need to set env variable, KUBECONFIG=<path-to-kubeconfig>/.kubeconfig
	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})

	// get the kubevirt client, using which kubevirt resources can be managed.
	vc, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	if err != nil {
		log.Fatal().AnErr("cannot obtain KubeVirt client: %v\n", err)
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		log.Fatal().AnErr("cannot obtain K8S Rest Config: %v\n", err)
	}
	config.QPS = Global.KubernetesConfig.QPS
	config.Burst = Global.KubernetesConfig.Burst
	config.RateLimiter = nil

	k8sConfig, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal().AnErr("cannot obtain K8S ClientSet: %v\n", err)
	}

	KubeOvnClient, err = clientset.NewForConfig(config)
	if err != nil {
		log.Fatal().AnErr("cannot obtain KubeOVN ClientSet: %v\n", err)
	}

	SnapshotClient, err = snapshot.NewForConfig(config)
	if err != nil {
		log.Fatal().AnErr("cannot obtain Snapshot Client: %v\n", err)
	}

	InformerClient, err = dynamic.NewForConfig(config)
	if err != nil {
		log.Fatal().AnErr("cannot obtain Informer Client: %v\n", err)
	}

	VirtClient = vc
	K8sClient = k8sConfig

	DynamicClientSet, err = dynamic.NewForConfig(config)
	if err != nil {
		log.Fatal().AnErr("cannot obtain DynamicClientSet: %v\n", err)
	}

}
