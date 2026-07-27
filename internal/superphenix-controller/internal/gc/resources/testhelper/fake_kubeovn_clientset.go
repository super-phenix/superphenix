package testhelper

import (
	kubeovnv1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	clientset "github.com/kubeovn/kube-ovn/pkg/client/clientset/versioned"
	kubeovnv1client "github.com/kubeovn/kube-ovn/pkg/client/clientset/versioned/typed/kubeovn/v1"
	fakekubeovnv1 "github.com/kubeovn/kube-ovn/pkg/client/clientset/versioned/typed/kubeovn/v1/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/testing"
)

var kubeovnScheme = runtime.NewScheme()
var kubeovnCodecs = serializer.NewCodecFactory(kubeovnScheme)

func init() {
	utilruntime.Must(kubeovnv1.AddToScheme(kubeovnScheme))
	metav1.AddToGroupVersion(kubeovnScheme, schema.GroupVersion{Version: "v1"})
}

// FakeKubeOvnClientset implements clientset.Interface without using the
// deprecated fake.NewSimpleClientset function.
type FakeKubeOvnClientset struct {
	testing.Fake
	discovery *fakediscovery.FakeDiscovery
	tracker   testing.ObjectTracker
}

var (
	_ clientset.Interface = &FakeKubeOvnClientset{}
	_ testing.FakeClient  = &FakeKubeOvnClientset{}
)

// NewFakeKubeOvnClientset creates a fake KubeOVN clientset backed by an object tracker.
// This replaces the deprecated kubeovnfake.NewSimpleClientset.
func NewFakeKubeOvnClientset(objects ...runtime.Object) *FakeKubeOvnClientset {
	o := testing.NewObjectTracker(kubeovnScheme, kubeovnCodecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	cs := &FakeKubeOvnClientset{tracker: o}
	cs.discovery = &fakediscovery.FakeDiscovery{Fake: &cs.Fake}
	cs.AddReactor("*", "*", testing.ObjectReaction(o))
	cs.AddWatchReactor("*", func(action testing.Action) (handled bool, ret watch.Interface, err error) {
		var opts metav1.ListOptions
		if watchAction, ok := action.(testing.WatchActionImpl); ok {
			opts = watchAction.ListOptions
		}
		gvr := action.GetResource()
		ns := action.GetNamespace()
		w, err := o.Watch(gvr, ns, opts)
		if err != nil {
			return false, nil, err
		}
		return true, w, nil
	})

	return cs
}

func (c *FakeKubeOvnClientset) Discovery() discovery.DiscoveryInterface {
	return c.discovery
}

func (c *FakeKubeOvnClientset) Tracker() testing.ObjectTracker {
	return c.tracker
}

func (c *FakeKubeOvnClientset) KubeovnV1() kubeovnv1client.KubeovnV1Interface {
	return &fakekubeovnv1.FakeKubeovnV1{Fake: &c.Fake}
}
