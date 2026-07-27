package loadBalancer

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListLoadBalancer(ctx context.Context, namespace string) []view.LBView {
	list := informers.WatcherSet[informers.SwitchLBRules].List()
	lbs := make([]view.LBView, 0)
	for _, item := range list {
		lb := item.(*unstructured.Unstructured)
		if lb.GetLabels()[spxId.SpxLabelProjectID] == namespace {
			lbView := view.UnstructuredLBToView(lb)
			lbs = append(lbs, lbView)
		}
	}

	return lbs
}
