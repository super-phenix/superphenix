package snat

import (
	"context"
	"strings"

	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/google/uuid"
	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateSNAT(ctx context.Context, internalCIDR string, eipMetadata spxId.Metadata) error {
	internalCIDR = strings.TrimSpace(internalCIDR)

	snatMeta := spxId.Metadata{}
	eipEidUUID, err := uuid.Parse(eipMetadata.ResourceEffectiveId)
	if err != nil {
		log.Err(err).Msg("Failed to parse eip uuid")
		return err
	}

	localId := uuid.NewSHA1(
		eipEidUUID,
		[]byte(internalCIDR),
	)
	err = snatMeta.GenerateMetadata(eipMetadata.ProjectId, eipMetadata.OrgId, localId.String())
	if err != nil {
		log.Error().
			Str("localId", localId.String()).
			Err(err).Msg("Failed to create metadata for SNAT")
		return err
	}

	snatRule := v1.IptablesSnatRule{
		ObjectMeta: metav1.ObjectMeta{Name: snatMeta.GetResourceEffectiveID(), Labels: snatMeta.GetLabels()},
		Spec: v1.IptablesSnatRuleSpec{
			EIP:          eipMetadata.GetResourceEffectiveID(),
			InternalCIDR: internalCIDR,
		},
	}
	if _, err := k8s.KubeOvnClient.KubeovnV1().IptablesSnatRules().Create(ctx, &snatRule, metav1.CreateOptions{}); err != nil {
		log.Error().Any("snatRule", snatRule).Msg("Failed to create iptables snat")
		return err
	}
	return nil
}
