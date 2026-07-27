package view

import (
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type EIPView struct {
	v1.IptablesEIP `json:",inline"`
}

type FIPView struct {
	v1.IptablesFIPRule `json:",inline"`
}

type SNATView struct {
	v1.IptablesSnatRule `json:",inline"`
	// Property use to detect legacy SNAT - meant to be deleted
	Legacy bool `json:"legacy,omitempty"`
}

type DNATView struct {
	v1.IptablesDnatRule `json:",inline"`
}

func UnstructuredEipToView(eip *unstructured.Unstructured) EIPView {
	var view EIPView
	if err := utils.UnstructuredToStruct(eip, &view); err != nil {
		return EIPView{}
	}
	view.Labels = utils.FilterLabels(view.GetLabels())
	view.Annotations = utils.FilterAnnotations(view.GetAnnotations())
	return view
}

func EipToView(eip v1.IptablesEIP) EIPView {
	eip.Labels = utils.FilterLabels(eip.GetLabels())
	eip.Annotations = utils.FilterAnnotations(eip.GetAnnotations())

	return Transform[v1.IptablesEIP, EIPView](eip)
}

func FipToView(fip v1.IptablesFIPRule) *FIPView {
	fip.Labels = utils.FilterLabels(fip.GetLabels())
	fip.Annotations = utils.FilterAnnotations(fip.GetAnnotations())

	view := Transform[v1.IptablesFIPRule, FIPView](fip)
	return &view
}

func UnstructuredSnatToView(snat *unstructured.Unstructured) SNATView {
	var view SNATView
	if err := utils.UnstructuredToStruct(snat, &view); err != nil {
		return SNATView{}
	}
	view.Labels = utils.FilterLabels(view.GetLabels())
	view.Annotations = utils.FilterAnnotations(view.GetAnnotations())
	return view
}

func SnatToView(snat v1.IptablesSnatRule) SNATView {
	snat.Labels = utils.FilterLabels(snat.GetLabels())
	snat.Annotations = utils.FilterAnnotations(snat.GetAnnotations())

	view := Transform[v1.IptablesSnatRule, SNATView](snat)
	return view
}

func UnstructuredDnatToView(dnat *unstructured.Unstructured) DNATView {
	var view DNATView
	if err := utils.UnstructuredToStruct(dnat, &view); err != nil {
		return DNATView{}
	}
	view.Labels = utils.FilterLabels(view.GetLabels())
	view.Annotations = utils.FilterAnnotations(view.GetAnnotations())
	return view
}

func EIPViewToResource(eipView EIPView, fip *FIPView, snat []SNATView, dnat []DNATView) EIP {
	return EIP{
		Resource: Resource{
			ID:          eipView.Labels[spxId.SpxLabelResourceLocalID],
			EId:         eipView.Name,
			ProductName: eipView.Labels[spxId.SpxLabelResourceName],
			Gitops:      eipView.Labels[spxId.SpxLabelGitops],
		},
		EIP:  eipView,
		FIP:  fip,
		SNAT: snat,
		DNAT: dnat,
	}
}
