package pvc

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListPVCs(ctx context.Context, namespace string) []view.PVCView {
	log := logger.GetLogger(ctx)
	if namespace == "" {
		log.Error().Msg("No namespace provided")
		return make([]view.PVCView, 0)
	}

	list, err := informers.WatcherSet[informers.PersistentVolumeClaims].ByIndex("namespace", namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error listing pvcs")
		return make([]view.PVCView, 0)
	}

	pvcs := make([]view.PVCView, 0)
	for _, item := range list {
		pvc := item.(*unstructured.Unstructured)
		pvcView := view.UnstructuredPVCToView(pvc)
		pvcs = append(pvcs, pvcView)
	}

	return pvcs
}
