package testhelper

import (
	k8scnicncfiov1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/testing"
	clientset "kubevirt.io/client-go/networkattachmentdefinitionclient"
	k8scnicncfiov1client "kubevirt.io/client-go/networkattachmentdefinitionclient/typed/k8s.cni.cncf.io/v1"
	fakek8scnicncfiov1 "kubevirt.io/client-go/networkattachmentdefinitionclient/typed/k8s.cni.cncf.io/v1/fake"
)

var nadScheme = runtime.NewScheme()
var nadCodecs = serializer.NewCodecFactory(nadScheme)

func init() {
	utilruntime.Must(k8scnicncfiov1.AddToScheme(nadScheme))
	metav1.AddToGroupVersion(nadScheme, schema.GroupVersion{Version: "v1"})
}

// FakeNadClientset implements clientset.Interface without using the
// deprecated fake.NewSimpleClientset function.
type FakeNadClientset struct {
	testing.Fake
	discovery *fakediscovery.FakeDiscovery
	tracker   testing.ObjectTracker
}

var (
	_ clientset.Interface = &FakeNadClientset{}
	_ testing.FakeClient  = &FakeNadClientset{}
)

// NewFakeNadClientset creates a fake NAD clientset backed by an object tracker.
// This replaces the deprecated nadfake.NewSimpleClientset.
func NewFakeNadClientset(objects ...runtime.Object) *FakeNadClientset {
	o := testing.NewObjectTracker(nadScheme, nadCodecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	cs := &FakeNadClientset{tracker: o}
	cs.discovery = &fakediscovery.FakeDiscovery{Fake: &cs.Fake}
	cs.AddReactor("*", "*", testing.ObjectReaction(o))
	cs.AddWatchReactor("*", func(action testing.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		w, err := o.Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		return true, w, nil
	})

	return cs
}

func (c *FakeNadClientset) Discovery() discovery.DiscoveryInterface {
	return c.discovery
}

func (c *FakeNadClientset) Tracker() testing.ObjectTracker {
	return c.tracker
}

func (c *FakeNadClientset) K8sCniCncfIoV1() k8scnicncfiov1client.K8sCniCncfIoV1Interface {
	return &fakek8scnicncfiov1.FakeK8sCniCncfIoV1{Fake: &c.Fake}
}
