package dnat

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func List(ctx context.Context, namespace, eip string) ([]view.DNATView, error) {
	list := informers.WatcherSet[informers.DNAT].List()
	dnats := make([]view.DNATView, 0)
	for _, item := range list {
		dnat := item.(*unstructured.Unstructured)
		if dnat.GetLabels()[spxId.SpxLabelProjectID] == namespace {
			dnatView := view.UnstructuredDnatToView(dnat)

			if dnatView.Spec.EIP == eip {
				dnats = append(dnats, dnatView)
			}
		}
	}

	return dnats, nil
}

func ListResources(ctx context.Context, namespace, eip string) ([]v1.IptablesDnatRule, error) {
	log := logger.GetLogger(ctx)
	projectLabel := fmt.Sprintf("%s=%s", spxId.SpxLabelProjectID, namespace)

	dnatList, err := k8s.KubeOvnClient.KubeovnV1().IptablesDnatRules().List(ctx, metav1.ListOptions{
		LabelSelector: projectLabel,
	})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error listing Dnat resources")
		return make([]v1.IptablesDnatRule, 0), err
	}

	dnatFiltered := make([]v1.IptablesDnatRule, 0)
	for _, item := range dnatList.Items {
		if item.Spec.EIP == eip {
			dnatFiltered = append(dnatFiltered, item)
		}
	}

	return dnatFiltered, nil
}
