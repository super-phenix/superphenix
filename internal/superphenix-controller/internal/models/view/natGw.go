package view

import (
	"encoding/json"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	"github.com/rs/zerolog/log"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type NatGwView struct {
	k8smetav1.TypeMeta `json:",inline"`
	ObjectMeta         `json:"metadata,omitempty"`

	Spec   VpcNatSpec   `json:"spec"`
	Status VpcNatStatus `json:"status,omitempty"`
}

type VpcNatSpec struct {
	Vpc             string   `json:"vpc"`
	Subnet          string   `json:"subnet"`
	ExternalSubnets []string `json:"externalSubnets"`
	LanIP           string   `json:"lanIp"`
	QoSPolicy       string   `json:"qosPolicy"`
}

type VpcNatStatus struct {
	QoSPolicy       string   `json:"qosPolicy" patchStrategy:"merge"`
	ExternalSubnets []string `json:"externalSubnets" patchStrategy:"merge"`
}

func UnstructuredNatGwToView(natGw *unstructured.Unstructured) NatGwView {
	var view NatGwView
	if err := utils.UnstructuredToStruct(natGw, &view); err != nil {
		return NatGwView{}
	}
	view.Labels = utils.FilterLabels(view.Labels)
	view.Annotations = utils.FilterAnnotations(view.Annotations)
	return view
}

func NatGwToView(natGw v1.VpcNatGateway) *NatGwView {
	natGw.Labels = utils.FilterLabels(natGw.GetLabels())
	natGw.Annotations = utils.FilterAnnotations(natGw.GetAnnotations())

	natGwStr, err := json.Marshal(natGw)
	if err != nil {
		log.Error().AnErr("error converting natGw to string", err).Any("natGw", natGw).Send()
	}
	var view NatGwView
	err = json.Unmarshal(natGwStr, &view)
	if err != nil {
		log.Error().AnErr("error converting string to view", err).Any("natGw", natGw).Send()
	}
	return &view
}
