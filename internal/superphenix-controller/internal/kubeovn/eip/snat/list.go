package snat

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

func List(ctx context.Context, namespace, eip string) ([]view.SNATView, error) {
	log := logger.GetLogger(ctx)
	// Try to get single SNAT for compatibility reason
	oldSnat, err := Get(ctx, eip)
	// We found an oldSNAT, return it
	if oldSnat != nil && err == nil {
		return []view.SNATView{{
			IptablesSnatRule: oldSnat.IptablesSnatRule,
			Legacy:           true,
		}}, nil
	} else if err != nil {
		log.Err(err).Str("eip", eip).Str("namespace", namespace).Msg("Failed to retrieve old SNAT")
		return []view.SNATView{}, err
	}
	// Else fetch the SNAT list
	list := informers.WatcherSet[informers.SNAT].List()
	snats := make([]view.SNATView, 0)
	for _, item := range list {
		snat := item.(*unstructured.Unstructured)
		if snat.GetLabels()[spxId.SpxLabelProjectID] == namespace {
			snatView := view.UnstructuredSnatToView(snat)

			if snatView.Spec.EIP == eip {
				snats = append(snats, snatView)
			}
		}
	}

	return snats, nil
}

func ListResources(ctx context.Context, namespace, eip string) ([]v1.IptablesSnatRule, error) {
	log := logger.GetLogger(ctx)

	// Try to get single SNAT for compatibility reason
	oldSnat, err := k8s.KubeOvnClient.KubeovnV1().IptablesSnatRules().Get(ctx, eip, metav1.GetOptions{})
	// We found an oldSNAT, return it
	if oldSnat != nil && err == nil {
		return []v1.IptablesSnatRule{*oldSnat}, nil
	} else if err != nil && !apierrors.IsNotFound(err) {
		log.Err(err).Str("eip", eip).Str("namespace", namespace).Msg("Failed to retrieve old SNAT")
		return []v1.IptablesSnatRule{}, err
	}

	projectLabel := fmt.Sprintf("%s=%s", spxId.SpxLabelProjectID, namespace)

	snatList, err := k8s.KubeOvnClient.KubeovnV1().IptablesSnatRules().List(ctx, metav1.ListOptions{
		LabelSelector: projectLabel,
	})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error listing SNAT resources")
		return make([]v1.IptablesSnatRule, 0), err
	}

	snatFiltered := make([]v1.IptablesSnatRule, 0)
	for _, item := range snatList.Items {
		if item.Spec.EIP == eip {
			snatFiltered = append(snatFiltered, item)
		}
	}

	return snatFiltered, nil
}
