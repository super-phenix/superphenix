package eip

import (
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListEips(namespace string) []view.EIPView {
	list := informers.WatcherSet[informers.EIP].List()
	eips := make([]view.EIPView, 0)
	for _, item := range list {
		eip := item.(*unstructured.Unstructured)
		if eip.GetLabels()[spxId.SpxLabelProjectID] == namespace {
			eipView := view.UnstructuredEipToView(eip)
			eips = append(eips, eipView)
		}
	}
	return eips
}
