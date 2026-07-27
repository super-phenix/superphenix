package netpol

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "k8s.io/api/networking/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreateNetPolInfo struct {
	spxId.Metadata
	Description string        `json:"description"`
	Target      LabelSelector `json:"target"`
	Ingress     []IngressRule `json:"ingress"`
	Egress      []EgressRule  `json:"egress"`
}

func (info *CreateNetPolInfo) CreateNetPol(ctx context.Context, namespace string) error {
	log := logger.GetLogger(ctx)
	if err := info.Metadata.ConvertToSpxMetadata(info.Metadata); err != nil {
		log.Err(err).Str("namespace", namespace).Any("info", info).Msg("Failed to parse metadata")
		return err
	}

	defaultNetPolAnnotations := map[string]string{
		"ovn.kubernetes.io/network_policy_enforcement":     "lax",
		fmt.Sprintf("%sdescription", spxId.SpxLabelPrefix): info.Description,
	}

	netPol := v1.NetworkPolicy{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:        info.Metadata.GetResourceEffectiveID(),
			Labels:      info.Metadata.GetLabels(),
			Annotations: defaultNetPolAnnotations,
		},
		Spec: v1.NetworkPolicySpec{
			PolicyTypes: []v1.PolicyType{},
		},
	}

	if err := withPodSelection(ctx, &netPol, info.Target, info.GetProjectID()); err != nil {
		log.Err(err).Any("info", info).Str("namespace", namespace).Msg("Failed to add pod selection")
		return err
	}

	if err := withIngress(ctx, &netPol, info.Ingress, info.GetProjectID()); err != nil {
		log.Err(err).Any("info", info).Str("namespace", namespace).Msg("Failed to add ingress rules")
		return err
	}

	if err := withEgress(ctx, &netPol, info.Egress, info.GetProjectID()); err != nil {
		log.Err(err).Any("info", info).Str("namespace", namespace).Msg("Failed to add egress rules")
		return err
	}

	_, err := config.K8sClient.NetworkingV1().NetworkPolicies(namespace).Create(ctx, &netPol, k8smetav1.CreateOptions{})
	if err != nil {
		log.Err(err).Any("netPol", netPol).Str("namespace", namespace).Msg("Failed to create network policy")
		return err
	}
	return nil
}
