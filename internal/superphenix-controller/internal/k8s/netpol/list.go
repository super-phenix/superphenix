package netpol

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListNetPols(ctx context.Context, namespace string) ([]view.NetPolView, error) {
	log := logger.GetLogger(ctx)
	if namespace == "" {
		log.Error().Msg("No namespace provided")
		return make([]view.NetPolView, 0), fmt.Errorf("no namespace provided")
	}

	list, err := informers.WatcherSet[informers.NetworkPolicy].ByIndex("namespace", namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error listing net pol")
		return make([]view.NetPolView, 0), err
	}

	netpols := make([]view.NetPolView, 0)
	for _, item := range list {
		netpol := item.(*unstructured.Unstructured)
		netpolView := view.UnstructuredNetPolToView(netpol)
		netpols = append(netpols, netpolView)
	}
	return netpols, nil
}
