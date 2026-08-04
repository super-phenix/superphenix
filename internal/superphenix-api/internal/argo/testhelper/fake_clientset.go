package testhelper

import (
	clientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	argoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/typed/application/v1alpha1"
	fakeargoprojv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/typed/application/v1alpha1/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/testing"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	applicationv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var scheme = runtime.NewScheme()
var codecs = serializer.NewCodecFactory(scheme)

func init() {
	utilruntime.Must(applicationv1alpha1.AddToScheme(scheme))
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Version: "v1"})
}

// FakeClientset implements clientset.Interface without using the deprecated
// fake.NewSimpleClientset function. It is backed by a simple object tracker.
type FakeClientset struct {
	testing.Fake
	discovery *fakediscovery.FakeDiscovery
	tracker   testing.ObjectTracker
}

var (
	_ clientset.Interface = &FakeClientset{}
	_ testing.FakeClient  = &FakeClientset{}
)

// NewFakeClientset creates a fake ArgoCD clientset backed by an object tracker.
// This replaces the deprecated fake.NewSimpleClientset.
func NewFakeClientset(objects ...runtime.Object) *FakeClientset {
	o := testing.NewObjectTracker(scheme, codecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	cs := &FakeClientset{tracker: o}
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

func (c *FakeClientset) Discovery() discovery.DiscoveryInterface {
	return c.discovery
}

func (c *FakeClientset) Tracker() testing.ObjectTracker {
	return c.tracker
}

func (c *FakeClientset) ArgoprojV1alpha1() argoprojv1alpha1.ArgoprojV1alpha1Interface {
	return &fakeargoprojv1alpha1.FakeArgoprojV1alpha1{Fake: &c.Fake}
}
