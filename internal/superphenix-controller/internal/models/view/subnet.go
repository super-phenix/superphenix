package view

import (
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type SubnetView struct {
	k8smetav1.TypeMeta `json:",inline"`
	ObjectMeta         `json:"metadata,omitempty"`

	IsShared bool `json:"isShared,omitempty"`

	Spec   SubnetSpec   `json:"spec"`
	Status SubnetStatus `json:"status,omitempty"`
}

type SubnetSpec struct {
	Default    bool     `json:"default"`
	Vpc        string   `json:"vpc,omitempty"`
	Protocol   string   `json:"protocol,omitempty"`
	Namespaces []string `json:"namespaces,omitempty"`
	CIDRBlock  string   `json:"cidrBlock"`
	Gateway    string   `json:"gateway"`
	ExcludeIps []string `json:"excludeIps,omitempty"`

	GatewayType string `json:"gatewayType,omitempty"`
	GatewayNode string `json:"gatewayNode"`
	NatOutgoing bool   `json:"natOutgoing"`

	DHCPv4Options string `json:"dhcpV4Options,omitempty"`
	DHCPv6Options string `json:"dhcpV6Options,omitempty"`

	Private bool `json:"private"`

	NatOutgoingPolicyRules []v1.NatOutgoingPolicyRule `json:"natOutgoingPolicyRules,omitempty"`
}

type SubnetStatus struct {
	// Conditions represents the latest state of the object
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []v1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	V4AvailableIPs         float64                          `json:"v4availableIPs"`
	V4AvailableIPRange     string                           `json:"v4availableIPrange"`
	V4UsingIPs             float64                          `json:"v4usingIPs"`
	V4UsingIPRange         string                           `json:"v4usingIPrange"`
	V6AvailableIPs         float64                          `json:"v6availableIPs"`
	V6AvailableIPRange     string                           `json:"v6availableIPrange"`
	V6UsingIPs             float64                          `json:"v6usingIPs"`
	V6UsingIPRange         string                           `json:"v6usingIPrange"`
	ActivateGateway        string                           `json:"activateGateway"`
	NatOutgoingPolicyRules []v1.NatOutgoingPolicyRuleStatus `json:"natOutgoingPolicyRules"`
}

func UnstructuredSubnetToView(subnet *unstructured.Unstructured, isShared bool) SubnetView {
	var view SubnetView
	if err := utils.UnstructuredToStruct(subnet, &view); err != nil {
		return SubnetView{}
	}
	view.Labels = utils.FilterLabels(view.Labels)
	view.Annotations = utils.FilterAnnotations(view.Annotations)
	view.IsShared = isShared
	return view
}

func SubnetToView(subnet v1.Subnet, isShared bool) SubnetView {
	subnet.Labels = utils.FilterLabels(subnet.GetLabels())
	subnet.Annotations = utils.FilterAnnotations(subnet.GetAnnotations())

	subView := Transform[v1.Subnet, SubnetView](subnet)
	subView.IsShared = isShared
	return subView
}

func SubnetViewToResource(subnetView SubnetView, natGw *NatGwView) Subnet {
	return Subnet{
		Resource: Resource{
			ID:          subnetView.Labels[spxId.SpxLabelResourceLocalID],
			EId:         subnetView.Name,
			ProductName: subnetView.Labels[spxId.SpxLabelResourceName],
			Gitops:      subnetView.Labels[spxId.SpxLabelGitops],
		},
		Subnet:     subnetView,
		NatGateway: natGw,
	}
}
