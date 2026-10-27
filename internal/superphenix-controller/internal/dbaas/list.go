package dbaas

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListDBaaS(ctx context.Context, namespace string) ([]view.DBaaS, error) {
	log := logger.GetLogger(ctx)

	watcher, ok := informers.WatcherSet[informers.PostgresCluster]
	if !ok {
		return nil, fmt.Errorf("cnpg cluster informer not initialized")
	}

	list, err := watcher.ByIndex("namespace", namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error ListDBaaS")
		return nil, err
	}

	databases := make([]view.DBaaS, 0, len(list))
	for _, item := range list {
		cluster, ok := item.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		if cluster.GetLabels()[spxId.SpxLabelProjectID] != namespace {
			continue
		}
		databases = append(databases, clusterToResource(ctx, cluster))
	}

	return databases, nil
}
