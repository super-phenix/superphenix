package testhelper

import (
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"

	"k8s.io/client-go/tools/cache"
)

// SetupFakeWatcher populates informers.WatcherSet with a fake cache indexer
// containing the given objects for the specified resource type.
func SetupFakeWatcher(resourceType string, objects ...interface{}) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, obj := range objects {
		_ = indexer.Add(obj)
	}
	informers.WatcherSet[resourceType] = informers.Watcher{Indexer: indexer}
}
