package testhelper

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/testing"
	clientset "kubevirt.io/client-go/containerizeddataimporter"
	cdiv1beta1 "kubevirt.io/client-go/containerizeddataimporter/typed/core/v1beta1"
	fakecdiv1beta1 "kubevirt.io/client-go/containerizeddataimporter/typed/core/v1beta1/fake"
	uploadv1beta1 "kubevirt.io/client-go/containerizeddataimporter/typed/upload/v1beta1"
	fakeuploadv1beta1 "kubevirt.io/client-go/containerizeddataimporter/typed/upload/v1beta1/fake"

	cdiapisv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cdiuploadapisv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/upload/v1beta1"
)

var cdiScheme = runtime.NewScheme()
var cdiCodecs = serializer.NewCodecFactory(cdiScheme)

func init() {
	utilruntime.Must(cdiapisv1beta1.AddToScheme(cdiScheme))
	utilruntime.Must(cdiuploadapisv1beta1.AddToScheme(cdiScheme))
	metav1.AddToGroupVersion(cdiScheme, schema.GroupVersion{Version: "v1"})
}

// FakeCdiClientset implements clientset.Interface without using the
// deprecated fake.NewSimpleClientset function.
type FakeCdiClientset struct {
	testing.Fake
	discovery *fakediscovery.FakeDiscovery
	tracker   testing.ObjectTracker
}

var (
	_ clientset.Interface = &FakeCdiClientset{}
	_ testing.FakeClient  = &FakeCdiClientset{}
)

// NewFakeCdiClientset creates a fake CDI clientset backed by an object tracker.
// This replaces the deprecated cdifake.NewSimpleClientset.
func NewFakeCdiClientset(objects ...runtime.Object) *FakeCdiClientset {
	o := testing.NewObjectTracker(cdiScheme, cdiCodecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	cs := &FakeCdiClientset{tracker: o}
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

func (c *FakeCdiClientset) Discovery() discovery.DiscoveryInterface {
	return c.discovery
}

func (c *FakeCdiClientset) Tracker() testing.ObjectTracker {
	return c.tracker
}

func (c *FakeCdiClientset) CdiV1beta1() cdiv1beta1.CdiV1beta1Interface {
	return &fakecdiv1beta1.FakeCdiV1beta1{Fake: &c.Fake}
}

func (c *FakeCdiClientset) UploadV1beta1() uploadv1beta1.UploadV1beta1Interface {
	return &fakeuploadv1beta1.FakeUploadV1beta1{Fake: &c.Fake}
}
