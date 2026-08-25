package netpol

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/subnet"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "k8s.io/api/networking/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UpdateNetPolInfo struct {
	Description string        `json:"description"`
	Target      LabelSelector `json:"target"`
	Ingress     []IngressRule `json:"ingress"`
	Egress      []EgressRule  `json:"egress"`
	// SubnetEIds scopes the policy to these subnets, empty meaning all of them.
	SubnetEIds []string `json:"subnetEIds" validate:"max=10"`
}

func (info *UpdateNetPolInfo) UpdateNetPol(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)

	netPol, err := config.K8sClient.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Any("info", info).Msg("Failed to get update")
		return fmt.Errorf("failed to get disk")
	}

	if err := utils.CheckProjectLabel(netPol, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("NetworkPolicy access denied")
		return err
	}

	if err := utils.IsEditAllowed(netPol.GetLabels()); err != nil {
		log.Error().Err(err).Any("info", info).Str("namespace", namespace).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	annotations := netPol.Annotations
	annotations[fmt.Sprintf("%sdescription", spxId.SpxLabelPrefix)] = info.Description

	netPol.Annotations = annotations

	if err := withPodSelection(ctx, netPol, info.Target, namespace); err != nil {
		log.Err(err).Any("info", info).Str("namespace", namespace).Msg("Failed to add pod selection")
		return err
	}

	// Reset policy type
	netPol.Spec.PolicyTypes = make([]v1.PolicyType, 0)

	if err := withIngress(ctx, netPol, info.Ingress, namespace); err != nil {
		log.Err(err).Any("info", info).Str("namespace", namespace).Msg("Failed to add ingress rules")
		return err
	}

	if err := withEgress(ctx, netPol, info.Egress, namespace); err != nil {
		log.Err(err).Any("info", info).Str("namespace", namespace).Msg("Failed to add egress rules")
		return err
	}

	if err := withSubnetScope(netPol, namespace, info.SubnetEIds, subnet.ListSubnet(ctx, namespace)); err != nil {
		log.Err(err).Any("info", info).Str("namespace", namespace).Msg("Failed to scope the network policy to the subnets")
		return err
	}

	_, err = config.K8sClient.NetworkingV1().NetworkPolicies(namespace).Update(ctx, netPol, k8smetav1.UpdateOptions{})
	if err != nil {
		log.Err(err).Any("netPol", *netPol).Str("namespace", namespace).Msg("Failed to update network policy")
		return err
	}
	return nil
}
