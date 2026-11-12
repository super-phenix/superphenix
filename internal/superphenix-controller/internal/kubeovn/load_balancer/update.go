package loadBalancer

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UpdateLoadBalancerInfo struct {
	VIP string `json:"vip"`
	// Subnet the load balancer attaches to, endpoints must be in it
	SubnetEId string `json:"subnetEId"`
	// Selectors and Endpoints are mutually exclusives
	Selectors []string `json:"selectors"`
	Endpoints []string `json:"endpoints"` // ip list
	Ports     []Port   `json:"ports"`
}

func (info *UpdateLoadBalancerInfo) UpdateLoadBalancer(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)
	lbToUpdate, err := k8s.KubeOvnClient.KubeovnV1().SwitchLBRules().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Any("info", info).Msg("Failed to get Load Balancer")
		return err
	}

	if err := utils.CheckProjectLabel(lbToUpdate, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("LoadBalancer access denied")
		return err
	}

	if err := utils.IsEditAllowed(lbToUpdate.GetLabels()); err != nil {
		log.Error().Err(err).Any("info", info).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	pinned := lbToUpdate.GetAnnotations()[logicalSwitchAnnotation]
	if pinned == "" {
		return apierrors.NewBadRequest(fmt.Sprintf("load balancer %q is not attached to a subnet, delete and recreate it", name))
	}

	spec, err := buildSpec(info.VIP, namespace, info.Selectors, info.Endpoints, info.Ports)
	if err != nil {
		log.Err(err).Any("info", info).Msg("Error parsing update values")
		return err
	}

	subnet, err := getRuleSubnet(ctx, info.SubnetEId, namespace)
	if err != nil {
		log.Err(err).Any("info", info).Msg("Error getting load balancer subnet")
		return err
	}

	if pinned != subnet.Name {
		return apierrors.NewBadRequest(fmt.Sprintf("subnet of load balancer %q can't be changed", name))
	}

	if err := checkEndpointsInSubnet(subnet, spec.Endpoints); err != nil {
		log.Err(err).Any("info", info).Msg("Load balancer endpoints not in subnet")
		return err
	}

	if err := checkVIPAvailable(ctx, info.VIP, subnet.Spec.Vpc, name); err != nil {
		log.Err(err).Any("info", info).Msg("Load balancer VIP not available")
		return err
	}

	lbToUpdate.Spec.Vip = spec.Vip
	lbToUpdate.Spec.Selector = spec.Selector
	lbToUpdate.Spec.Endpoints = spec.Endpoints
	lbToUpdate.Spec.Ports = spec.Ports

	if _, err := k8s.KubeOvnClient.KubeovnV1().SwitchLBRules().Update(ctx, lbToUpdate, metav1.UpdateOptions{}); err != nil {
		log.Err(err).Msg("Failed to update Load Balancer")
		return err
	}

	return nil
}
