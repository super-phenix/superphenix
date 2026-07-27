package dnat

import (
	"context"
	"fmt"
	"strings"

	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/google/uuid"
	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type InfoDNAT struct {
	ExternalPort string `json:"externalPort"`
	InternalIP   string `json:"internalIP"`
	InternalPort string `json:"internalPort"`
	Protocol     string `json:"protocol"`
}

func CreateDNAT(ctx context.Context, item InfoDNAT, eipMetadata spxId.Metadata) error {
	dnatMeta := spxId.Metadata{}
	eipEidUUID, err := uuid.Parse(eipMetadata.ResourceEffectiveId)
	if err != nil {
		log.Err(err).Msg("Failed to parse eip uuid")
		return err
	}

	externalPort := strings.TrimSpace(item.ExternalPort)
	internalIP := strings.TrimSpace(item.InternalIP)
	internalPort := strings.TrimSpace(item.InternalPort)

	localId := uuid.NewSHA1(
		eipEidUUID,
		[]byte(fmt.Sprintf("%s-%s-%s-%s", externalPort, internalIP, internalPort, item.Protocol)),
	)

	err = dnatMeta.GenerateMetadata(eipMetadata.ProjectId, eipMetadata.OrgId, localId.String())
	if err != nil {
		log.Err(err).
			Str("localId", localId.String()).
			Msg("Failed to create metadata for DNAT")
		return err
	}

	dnatRule := v1.IptablesDnatRule{
		ObjectMeta: metav1.ObjectMeta{Name: dnatMeta.GetResourceEffectiveID(), Labels: dnatMeta.GetLabels()},
		Spec: v1.IptablesDnatRuleSpec{
			EIP:          eipMetadata.GetResourceEffectiveID(),
			ExternalPort: externalPort,
			Protocol:     item.Protocol,
			InternalIP:   internalIP,
			InternalPort: internalPort,
		},
	}
	if _, err := k8s.KubeOvnClient.KubeovnV1().IptablesDnatRules().Create(ctx, &dnatRule, metav1.CreateOptions{}); err != nil {
		log.Error().Any("dnatRule", dnatRule).Msg("Failed to create iptables dnat")
		return err
	}

	return nil
}
