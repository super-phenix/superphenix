package informers

import (
	"context"
	"fmt"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	gcLog "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	k8scnicncfiov1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	kubeovn "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	volumesnapshot "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	v2 "kubevirt.io/api/core/v1"
	"kubevirt.io/api/snapshot/v1beta1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	bktv1alpha1 "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	veleroV1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// Kubevirt
const (
	VirtualMachine         = "virtualmachines"
	VirtualMachineInstance = "virtualmachineinstances"
	VirtualMachineSnapshot = "virtualmachinesnapshots"
	DataVolume             = "datavolumes"
	VolumeSnapshot         = "volumesnapshots"
)

// Kubeovn
const (
	EIP           = "iptables-eips"
	FIP           = "iptables-fip-rules"
	SNAT          = "iptables-snat-rules"
	DNAT          = "iptables-dnat-rules"
	VPC           = "vpcs"
	NatGw         = "vpc-nat-gateways"
	Subnet        = "subnets"
	NAD           = "network-attachment-definitions"
	SwitchLBRules = "switch-lb-rules"
)

// K8S
const (
	SSH                    = "secrets"
	Namespace              = "namespaces"
	PersistentVolumeClaims = "persistentvolumeclaims"
	NetworkPolicy          = "networkpolicies"
	Pods                   = "pods"
)

// BaaS
const (
	Backup   = "backups"
	Schedule = "schedules"
)

// S3
const (
	ObjectBucketClaim = "objectbucketclaims"
	ObjectBucket      = "objectbuckets"
	CephObjectStore   = "cephobjectstores"
)

type Watcher struct {
	cache.Indexer
}

var (
	resourceList = []schema.GroupVersionResource{
		// Kubevirt
		{Group: v2.SchemeGroupVersion.Group, Version: v2.SchemeGroupVersion.Version, Resource: VirtualMachine},
		{Group: v2.SchemeGroupVersion.Group, Version: v2.SchemeGroupVersion.Version, Resource: VirtualMachineInstance},
		{Group: v1beta1.SchemeGroupVersion.Group, Version: v1beta1.SchemeGroupVersion.Version, Resource: VirtualMachineSnapshot},
		{Group: cdiv1beta1.SchemeGroupVersion.Group, Version: cdiv1beta1.SchemeGroupVersion.Version, Resource: DataVolume},
		{Group: volumesnapshot.SchemeGroupVersion.Group, Version: volumesnapshot.SchemeGroupVersion.Version, Resource: VolumeSnapshot},
		// Kubeovn
		{Group: kubeovn.SchemeGroupVersion.Group, Version: kubeovn.SchemeGroupVersion.Version, Resource: EIP},
		{Group: kubeovn.SchemeGroupVersion.Group, Version: kubeovn.SchemeGroupVersion.Version, Resource: FIP},
		{Group: kubeovn.SchemeGroupVersion.Group, Version: kubeovn.SchemeGroupVersion.Version, Resource: SNAT},
		{Group: kubeovn.SchemeGroupVersion.Group, Version: kubeovn.SchemeGroupVersion.Version, Resource: DNAT},
		{Group: kubeovn.SchemeGroupVersion.Group, Version: kubeovn.SchemeGroupVersion.Version, Resource: VPC},
		{Group: kubeovn.SchemeGroupVersion.Group, Version: kubeovn.SchemeGroupVersion.Version, Resource: NatGw},
		{Group: kubeovn.SchemeGroupVersion.Group, Version: kubeovn.SchemeGroupVersion.Version, Resource: Subnet},
		{Group: k8scnicncfiov1.SchemeGroupVersion.Group, Version: k8scnicncfiov1.SchemeGroupVersion.Version, Resource: NAD},
		{Group: kubeovn.SchemeGroupVersion.Group, Version: kubeovn.SchemeGroupVersion.Version, Resource: SwitchLBRules},
		//	K8S
		{Group: corev1.SchemeGroupVersion.Group, Version: corev1.SchemeGroupVersion.Version, Resource: SSH},
		{Group: corev1.SchemeGroupVersion.Group, Version: corev1.SchemeGroupVersion.Version, Resource: Namespace},
		{Group: corev1.SchemeGroupVersion.Group, Version: corev1.SchemeGroupVersion.Version, Resource: PersistentVolumeClaims},
		{Group: networkingv1.SchemeGroupVersion.Group, Version: networkingv1.SchemeGroupVersion.Version, Resource: NetworkPolicy},
		{Group: corev1.SchemeGroupVersion.Group, Version: corev1.SchemeGroupVersion.Version, Resource: Pods},
		// BaaS
		{Group: veleroV1.SchemeGroupVersion.Group, Version: veleroV1.SchemeGroupVersion.Version, Resource: Backup},
		{Group: veleroV1.SchemeGroupVersion.Group, Version: veleroV1.SchemeGroupVersion.Version, Resource: Schedule},
		// S3
		{Group: bktv1alpha1.SchemeGroupVersion.Group, Version: bktv1alpha1.SchemeGroupVersion.Version, Resource: ObjectBucketClaim},
		{Group: bktv1alpha1.SchemeGroupVersion.Group, Version: bktv1alpha1.SchemeGroupVersion.Version, Resource: ObjectBucket},
		{Group: "ceph.rook.io", Version: "v1", Resource: CephObjectStore},
	}

	WatcherSet = map[string]Watcher{}
)

func InitResourceInformers(ctx context.Context) {
	logger := gcLog.GetProcessLogger(ctx)
	logger.Info().Msg("Initializing resource informers")
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(config.InformerClient, 10*time.Minute, corev1.NamespaceAll, nil)

	// For resources in list
	for _, res := range resourceList {
		logger.Info().Msgf("Starting informer %s", res.Resource)
		informer := factory.ForResource(res).Informer()
		go informer.Run(wait.NeverStop)
		WatcherSet[res.Resource] = Watcher{informer.GetIndexer()}
	}
}

func (w Watcher) ListMarkedResource(ctx context.Context) ([]k8smetav1.Object, error) {
	logger := gcLog.GetProcessLogger(ctx)
	list := w.List()

	filteredResources := make([]k8smetav1.Object, 0)
	for _, res := range list {
		objMeta, err := objectToMeta(res)

		if err != nil {
			logger.Error().Err(err).Msg("Failed to get object metadata")
		}

		if objMeta.GetLabels()[config.Global.GarbageCollection.LabelMarkKey] != "" {
			filteredResources = append(filteredResources, objMeta)
		}

	}

	return filteredResources, nil
}

func (w Watcher) ListNamespaceResource(ctx context.Context, namespace string) ([]k8smetav1.Object, error) {
	logger := gcLog.GetProcessLogger(ctx)
	list := w.List()

	filteredResources := make([]k8smetav1.Object, 0)
	for _, res := range list {
		objMeta, err := objectToMeta(res)

		if err != nil {
			logger.Error().Err(err).Msg("Failed to get object metadata")
		}

		if objMeta.GetLabels()[spxId.SpxLabelProjectID] == namespace {
			filteredResources = append(filteredResources, objMeta)
		}

	}

	return filteredResources, nil
}

// objectToMeta returns the structured metav1.Object
func objectToMeta(obj interface{}) (k8smetav1.Object, error) {
	objMeta, err := meta.Accessor(obj)
	if err != nil {
		return nil, fmt.Errorf("object has no meta: %v", err)
	}

	return objMeta, nil
}
