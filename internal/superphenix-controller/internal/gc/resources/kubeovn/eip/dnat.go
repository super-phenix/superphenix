package eip

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/alerting"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func ListMarkedDnat(ctx context.Context, c Cleaner) ([]utils.Resource, error) {
	watcher := informers.WatcherSet[informers.DNAT]
	dnatList, err := watcher.ListMarkedResource(ctx)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return make([]utils.Resource, 0), err
	}

	dnatFiltered := make([]utils.Resource, 0)
	for _, item := range dnatList {
		ok, err := utils.ParseTimestamp(item)
		if err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("name", item.GetName()).Str("deletionTimestamp", item.GetLabels()[config.Global.GarbageCollection.LabelMarkKey]).Msg("Failed to parse timestamp")
			alerting.RaiseAlert(ctx, alerting.ProcessCleaning, alerting.ErrorParsing, c.ResourceType, item)
			continue
		}

		if ok {
			dnatFiltered = append(dnatFiltered, item)
		}
	}

	return dnatFiltered, nil
}
