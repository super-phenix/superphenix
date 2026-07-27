package fip

import (
	"context"
	"strings"

	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateFip(ctx context.Context, internalIp string, eipMetadata spxId.Metadata) error {
	internalIP := strings.TrimSpace(internalIp)
	fip := v1.IptablesFIPRule{
		ObjectMeta: metav1.ObjectMeta{Name: eipMetadata.GetResourceEffectiveID(), Labels: eipMetadata.GetLabels()},
		Spec: v1.IptablesFIPRuleSpec{
			EIP:        eipMetadata.GetResourceEffectiveID(),
			InternalIP: internalIP,
		},
	}
	if _, err := k8s.KubeOvnClient.KubeovnV1().IptablesFIPRules().Create(ctx, &fip, metav1.CreateOptions{}); err != nil {
		log.Error().Any("fip", fip).Msg("Failed to create fip")
		return err
	}
	return nil
}
