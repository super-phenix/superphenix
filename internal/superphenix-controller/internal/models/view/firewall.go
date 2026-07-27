package view

import (
	"fmt"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/networking/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type NetPolView struct {
	k8smetav1.TypeMeta `json:",inline"`
	ObjectMeta         `json:"metadata,omitempty"`

	Spec        v1.NetworkPolicySpec `json:"spec"`
	Description string               `json:"description"`
}

func UnstructuredNetPolToView(fw *unstructured.Unstructured) NetPolView {
	var view NetPolView
	err := utils.UnstructuredToStruct(fw, &view)
	if err != nil {
		log.Error().AnErr("error converting netpol to view", err).Any("fw", fw).Send()
		return NetPolView{}
	}
	view.Labels = utils.FilterLabels(view.Labels)
	view.Annotations = utils.FilterAnnotations(view.Annotations)
	descAnnotations := fmt.Sprintf("%sdescription", spxId.SpxLabelPrefix)
	for k, v := range fw.GetAnnotations() {
		if strings.HasPrefix(k, descAnnotations) {
			view.Description = v
		}
	}
	return view
}

func NetPolToView(fw v1.NetworkPolicy) NetPolView {
	fw.Labels = utils.FilterLabels(fw.GetLabels())
	fw.Annotations = utils.FilterAnnotations(fw.GetAnnotations())

	fwView := Transform[v1.NetworkPolicy, NetPolView](fw)

	descAnnotations := fmt.Sprintf("%sdescription", spxId.SpxLabelPrefix)
	for k, v := range fw.GetAnnotations() {
		if strings.HasPrefix(k, descAnnotations) {
			fwView.Description = v
		}
	}

	return fwView
}

func (view NetPolView) ToResource() Firewall {
	return Firewall{
		Resource: Resource{
			ID:          view.Labels[spxId.SpxLabelResourceLocalID],
			EId:         view.Name,
			ProductName: view.Labels[spxId.SpxLabelResourceName],
			Gitops:      view.Labels[spxId.SpxLabelGitops],
		},
		Firewall: view,
	}
}
