package loadBalancer

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreateLoadBalancerInfo struct {
	spxId.Metadata
	VIP string `json:"vip"`
	// Subnet the load balancer attaches to, backends must be in it
	SubnetEId string `json:"subnetEId"`
	// Selectors and Endpoints are mutually exclusives
	Selectors []string `json:"selectors"`
	Endpoints []string `json:"endpoints"` // ip list
	Ports     []Port   `json:"ports"`
}

func (info *CreateLoadBalancerInfo) CreateLoadBalancer(ctx context.Context) error {
	log := logger.GetLogger(ctx)

	spec, selector, err := buildSpec(info.VIP, info.GetProjectID(), info.Selectors, info.Endpoints, info.Ports)
	if err != nil {
		log.Err(err).Any("info", info).Msg("Error parsing creation values")
		return err
	}

	subnet, err := getRuleSubnet(ctx, info.SubnetEId, info.GetProjectID())
	if err != nil {
		log.Err(err).Any("info", info).Msg("Error getting load balancer subnet")
		return err
	}

	if err := checkBackendsInSubnet(subnet, info.GetProjectID(), selector, spec.Endpoints); err != nil {
		log.Err(err).Any("info", info).Msg("Load balancer backends not in subnet")
		return err
	}

	if err := checkVIPAvailable(ctx, info.VIP, subnet.Spec.Vpc, info.GetResourceEffectiveID()); err != nil {
		log.Err(err).Any("info", info).Msg("Load balancer VIP not available")
		return err
	}

	_, err = config.KubeOvnClient.KubeovnV1().SwitchLBRules().Create(ctx, &v1.SwitchLBRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:   info.GetResourceEffectiveID(),
			Labels: info.GetLabels(),
			Annotations: map[string]string{
				logicalRouterAnnotation: subnet.Spec.Vpc,
				logicalSwitchAnnotation: subnet.Name,
			},
		},
		Spec: spec,
	}, metav1.CreateOptions{})
	return err
}
