package dbaas

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// listEndpoints returns the addresses of the SwitchLBRule the sfs-dbaas chart renders for a database.
func listEndpoints(ctx context.Context, namespace, localId string) []view.DBaaSEndpoint {
	log := logger.GetLogger(ctx)

	watcher, ok := informers.WatcherSet[informers.SwitchLBRules]
	if !ok {
		log.Warn().Msg("switch-lb-rules informer not initialized")
		return []view.DBaaSEndpoint{}
	}

	// SwitchLBRules are cluster-scoped
	for _, item := range watcher.List() {
		lb, ok := item.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		labels := lb.GetLabels()
		if labels[spxId.SpxLabelProjectID] == namespace && labels[spxId.SpxLabelResourceLocalID] == LBLocalIDPrefix+localId {
			return unstructuredToEndpoints(lb)
		}
	}

	return []view.DBaaSEndpoint{}
}

func clusterToResource(ctx context.Context, item *unstructured.Unstructured) view.DBaaS {
	cluster := unstructuredToCluster(item)
	endpoints := listEndpoints(ctx, item.GetNamespace(), item.GetLabels()[spxId.SpxLabelResourceLocalID])
	return view.DBaaSToResource(cluster, endpoints)
}
