package argocd

import (
	"context"
	"fmt"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Watcher handles dynamic watching of ArgoCD Applications.
type Watcher struct {
	client.Client
	Scheme     *runtime.Scheme
	RestMapper meta.RESTMapper
	Controller controller.Controller
	Cache      cache.Cache

	watchStarted atomic.Bool
}

// NewWatcher creates a new ArgoCD watcher.
func NewWatcher(c client.Client, scheme *runtime.Scheme, mapper meta.RESTMapper, ctrl controller.Controller, cache cache.Cache) *Watcher {
	return &Watcher{
		Client:     c,
		Scheme:     scheme,
		RestMapper: mapper,
		Controller: ctrl,
		Cache:      cache,
	}
}

// EnsureWatch checks if the ArgoCD CRDs are available and registers a watch for Applications.
// owner is the object type that owns the Applications (e.g., &operatorv1alpha1.Cluster{} or &corev1.ConfigMap{}).
func (w *Watcher) EnsureWatch(ctx context.Context, owner client.Object) {
	if w == nil || w.Controller == nil || w.Cache == nil || w.watchStarted.Load() {
		return
	}

	log := logf.FromContext(ctx)

	if err := CheckCRDs(ctx, w.RestMapper); err != nil {
		return
	}

	err := w.Controller.Watch(
		source.Kind(w.Cache, ApplicationMetadata(), handler.TypedEnqueueRequestForOwner[*metav1.PartialObjectMetadata](
			w.Scheme, w.RestMapper, owner,
		)),
	)
	if err != nil {
		log.Error(err, "Failed to start ArgoCD Application watch")
		return
	}

	w.watchStarted.Store(true)
	log.Info("Successfully started ArgoCD Application watch")
}

// ApplicationMetadata returns an empty metadata-only ArgoCD Application. Owner-based
// watches only need metadata, so the informer does not store spec or status.
func ApplicationMetadata() *metav1.PartialObjectMetadata {
	app := &metav1.PartialObjectMetadata{}
	app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	return app
}

// CheckCRDs verifies if the required ArgoCD CRDs are installed in the cluster.
func CheckCRDs(ctx context.Context, mapper meta.RESTMapper) error {
	if mapper == nil {
		return fmt.Errorf("rest mapper is nil")
	}
	gvks := []schema.GroupVersionKind{
		{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"},
		{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"},
	}

	for _, gvk := range gvks {
		_, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			if meta.IsNoMatchError(err) {
				return fmt.Errorf("ArgoCD CRD %s not found", gvk.Kind)
			}
			return err
		}
	}

	return nil
}
