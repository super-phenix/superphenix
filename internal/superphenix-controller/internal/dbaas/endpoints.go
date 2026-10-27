package dbaas

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// listEndpoints returns the addresses of the SwitchLBRule the sfs-dbaas chart renders for a database,
// followed by the ones on its EIP when external access is enabled.
func listEndpoints(ctx context.Context, namespace, localId string) []view.DBaaSEndpoint {
	lb := findRendered(ctx, informers.SwitchLBRules, namespace, LBLocalIDPrefix+localId)
	if lb == nil {
		return []view.DBaaSEndpoint{}
	}

	endpoints := unstructuredToEndpoints(lb)
	if eip := findRendered(ctx, informers.EIP, namespace, EIPLocalIDPrefix+localId); eip != nil {
		endpoints = append(endpoints, externalEndpoints(eip, endpoints)...)
	}

	return endpoints
}

// findRendered returns the object of a cluster-scoped informer the chart rendered for the project and localID.
func findRendered(ctx context.Context, resource, namespace, localId string) *unstructured.Unstructured {
	log := logger.GetLogger(ctx)

	watcher, ok := informers.WatcherSet[resource]
	if !ok {
		log.Warn().Str("resource", resource).Msg("informer not initialized")
		return nil
	}

	for _, item := range watcher.List() {
		obj, ok := item.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		labels := obj.GetLabels()
		if labels[spxId.SpxLabelProjectID] == namespace && labels[spxId.SpxLabelResourceLocalID] == localId {
			return obj
		}
	}

	return nil
}

func clusterToResource(ctx context.Context, item *unstructured.Unstructured) view.DBaaS {
	cluster := unstructuredToCluster(item)
	endpoints := listEndpoints(ctx, item.GetNamespace(), item.GetLabels()[spxId.SpxLabelResourceLocalID])
	return view.DBaaSToResource(cluster, endpoints)
}
