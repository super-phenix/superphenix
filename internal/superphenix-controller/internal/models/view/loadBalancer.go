package view

import (
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type LBView struct {
	k8smetav1.TypeMeta `json:",inline"`
	ObjectMeta         `json:"metadata,omitempty"`

	Spec   LBSpec   `json:"spec"`
	Status LBStatus `json:"status,omitempty"`
}

type LBSpec struct {
	Vip             string       `json:"vip"`
	Namespace       string       `json:"namespace"`
	Selector        []string     `json:"selector"`
	Endpoints       []string     `json:"endpoints"`
	SessionAffinity string       `json:"sessionAffinity,omitempty"`
	Ports           []LBRulePort `json:"ports"`
}

type LBRulePort struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort,omitempty"`
	Protocol   string `json:"protocol"`
}

type LBStatus struct {
	// Conditions represents the latest state of the object
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []v1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	Ports   string `json:"ports" patchStrategy:"merge"`
	Service string `json:"service" patchStrategy:"merge"`
}

func UnstructuredLBToView(lb *unstructured.Unstructured) LBView {
	var view LBView
	if err := utils.UnstructuredToStruct(lb, &view); err != nil {
		return LBView{}
	}
	view.Labels = utils.FilterLabels(view.Labels)
	view.Annotations = utils.FilterAnnotations(view.Annotations)
	return view
}

func LBToView(lb v1.SwitchLBRule) LBView {
	lb.Labels = utils.FilterLabels(lb.GetLabels())
	lb.Annotations = utils.FilterAnnotations(lb.GetAnnotations())

	return Transform[v1.SwitchLBRule, LBView](lb)
}

func (view LBView) ToResource() LoadBalancer {
	return LoadBalancer{
		Resource: Resource{
			ID:          view.Labels[spxId.SpxLabelResourceLocalID],
			EId:         view.Name,
			ProductName: view.Labels[spxId.SpxLabelResourceName],
			Gitops:      view.Labels[spxId.SpxLabelGitops],
		},
		LoadBalancer: view,
	}
}
