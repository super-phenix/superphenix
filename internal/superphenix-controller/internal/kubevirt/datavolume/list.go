package datavolume

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListDisks(ctx context.Context, namespace string) []view.DiskView {
	log := logger.GetLogger(ctx)
	if namespace == "" {
		log.Error().Msg("No namespace provided")
		return make([]view.DiskView, 0)
	}

	list, err := informers.WatcherSet[informers.DataVolume].ByIndex("namespace", namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error listing disks")
		return make([]view.DiskView, 0)
	}

	disks := make([]view.DiskView, 0)
	for _, item := range list {
		disk := item.(*unstructured.Unstructured)
		diskView := view.UnstructuredDiskToView(disk)
		disks = append(disks, diskView)
	}

	return disks
}
