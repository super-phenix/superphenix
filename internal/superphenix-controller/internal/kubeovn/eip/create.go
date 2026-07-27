package eip

import (
	"context"
	"fmt"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/eip/fip"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/google/uuid"
	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreateEIPInfo struct {
	spxId.Metadata
	General struct {
		SubnetEId string `json:"subnetEid"`
	} `json:"general"`
	Spec struct {
		InternalIP string   `json:"internalIP,omitempty"`
		SNAT       []string `json:"snat,omitempty"`
		DNAT       []struct {
			ExternalPort string `json:"externalPort"`
			InternalIP   string `json:"internalIP"`
			InternalPort string `json:"internalPort"`
			Protocol     string `json:"protocol"`
		} `json:"dnat,omitempty"`
	} `json:"spec"`
}

func (s *CreateEIPInfo) CreateEip(ctx context.Context) error {
	log := logger.GetLogger(ctx)
	namespace := s.GetProjectID()
	if err := s.Metadata.ConvertToSpxMetadata(s.Metadata); err != nil {
		log.Err(err).Str("namespace", namespace).Any("vpc", s).Msg("Failed to parse vpc info")
		return err
	}

	if s.Spec.InternalIP == "" && len(s.Spec.SNAT) == 0 {
		err := fmt.Errorf("no internalIP or SNAT specified")
		log.Err(err).Msg("Failed to create eip")
		return err
	}

	defaultAnnotations := make(map[string]string)
	defaultAnnotations["ovn.kubernetes.io/bgp"] = "true"

	eip := v1.IptablesEIP{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.GetResourceEffectiveID(),
			Labels:      s.GetLabels(),
			Annotations: defaultAnnotations,
		},
		Spec: v1.IptablesEIPSpec{
			NatGwDp:        s.General.SubnetEId,
			ExternalSubnet: k8s.Global.ProductsConfig.EipDefault.ExternalSubnet,
		},
	}
	if _, err := k8s.KubeOvnClient.KubeovnV1().IptablesEIPs().Create(ctx, &eip, metav1.CreateOptions{}); err != nil {
		log.Err(err).Any("info", s).Msg("Failed to create EIP")
		return err
	}

	if s.Spec.InternalIP != "" {
		if err := fip.CreateFip(ctx, s.Spec.InternalIP, s.Metadata); err != nil {
			log.Err(err).Any("info", s).Msg("Failed to create FIP, cleaning EIP")
			cleanEip(ctx, s.GetProjectID(), s.GetResourceEffectiveID())
			return err
		}

	} else if len(s.Spec.SNAT) != 0 {
		for _, internalCIDR := range s.Spec.SNAT {
			internalCIDR = strings.TrimSpace(internalCIDR)

			snatMeta := spxId.Metadata{}
			eipEidUUID, err := uuid.Parse(s.ResourceEffectiveId)
			if err != nil {
				log.Err(err).Msg("Failed to parse eip uuid")
				cleanEip(ctx, s.GetProjectID(), s.GetResourceEffectiveID())
				return err
			}

			localId := uuid.NewSHA1(
				eipEidUUID,
				[]byte(internalCIDR),
			)
			err = snatMeta.GenerateMetadata(s.ProjectId, s.OrgId, localId.String())
			if err != nil {
				log.Error().
					Str("localId", localId.String()).
					Err(err).Msg("Failed to create metadata for SNAT")
				cleanEip(ctx, s.GetProjectID(), s.GetResourceEffectiveID())
				return err
			}

			snat := v1.IptablesSnatRule{
				ObjectMeta: metav1.ObjectMeta{Name: snatMeta.GetResourceEffectiveID(), Labels: snatMeta.GetLabels()},
				Spec: v1.IptablesSnatRuleSpec{
					EIP:          s.GetResourceEffectiveID(),
					InternalCIDR: internalCIDR,
				},
			}
			if _, err := k8s.KubeOvnClient.KubeovnV1().IptablesSnatRules().Create(ctx, &snat, metav1.CreateOptions{}); err != nil {
				log.Err(err).Any("info", s).Msg("Failed to create SNAT, cleaning EIP")
				cleanEip(ctx, s.GetProjectID(), s.GetResourceEffectiveID())
				return err
			}
		}

		if s.Spec.DNAT != nil {
			for _, dnat := range s.Spec.DNAT {
				dnatMeta := spxId.Metadata{}
				eipEidUUID, err := uuid.Parse(s.ResourceEffectiveId)
				if err != nil {
					log.Err(err).Msg("Failed to parse eip uuid")
					cleanEip(ctx, s.GetProjectID(), s.GetResourceEffectiveID())
					return err
				}

				externalPort := strings.TrimSpace(dnat.ExternalPort)
				internalIP := strings.TrimSpace(dnat.InternalIP)
				internalPort := strings.TrimSpace(dnat.InternalPort)

				localId := uuid.NewSHA1(
					eipEidUUID,
					[]byte(fmt.Sprintf("%s-%s-%s-%s", externalPort, internalIP, internalPort, dnat.Protocol)),
				)
				err = dnatMeta.GenerateMetadata(s.ProjectId, s.OrgId, localId.String())
				if err != nil {
					log.Error().
						Str("localId", localId.String()).
						Err(err).Msg("Failed to create metadata for DNAT")
					cleanEip(ctx, s.GetProjectID(), s.GetResourceEffectiveID())
					return err
				}

				dnatRule := v1.IptablesDnatRule{
					ObjectMeta: metav1.ObjectMeta{Name: dnatMeta.GetResourceEffectiveID(), Labels: dnatMeta.GetLabels()},
					Spec: v1.IptablesDnatRuleSpec{
						EIP:          s.GetResourceEffectiveID(),
						ExternalPort: externalPort,
						Protocol:     dnat.Protocol,
						InternalIP:   internalIP,
						InternalPort: internalPort,
					},
				}

				if _, err := k8s.KubeOvnClient.KubeovnV1().IptablesDnatRules().Create(ctx, &dnatRule, metav1.CreateOptions{}); err != nil {
					log.Err(err).Any("info", s).Any("dnat", dnat).Msg("Failed to create DNAT, cleaning EIP")
					cleanEip(ctx, s.GetProjectID(), s.GetResourceEffectiveID())
					return err
				}
			}
		}
	}

	return nil
}
