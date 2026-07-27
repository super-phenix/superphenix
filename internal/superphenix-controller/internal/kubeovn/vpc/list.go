package vpc

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListVPC(ctx context.Context, namespace string) []view.VPCView {
	list := informers.WatcherSet[informers.VPC].List()
	vpcs := make([]view.VPCView, 0)
	for _, item := range list {
		vpc := item.(*unstructured.Unstructured)
		if vpc.GetLabels()[spxId.SpxLabelProjectID] == namespace {
			vpcView := view.UnstructuredVPCToView(vpc)
			vpcs = append(vpcs, vpcView)
		}
	}
	return vpcs
}
