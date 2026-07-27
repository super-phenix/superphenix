package eip

import (
	"context"
	"strings"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/eip/dnat"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/eip/fip"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/eip/snat"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UpdateEIPInfo struct {
	spxId.Metadata
	Spec struct {
		InternalIP string          `json:"internalIP,omitempty"`
		SNAT       []string        `json:"snat,omitempty"`
		DNAT       []dnat.InfoDNAT `json:"dnat,omitempty"`
	} `json:"spec"`
}

func (s *UpdateEIPInfo) UpdateEip(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)
	if err := s.Metadata.ConvertToSpxMetadata(s.Metadata); err != nil {
		log.Err(err).Str("namespace", namespace).Any("info", s).Msg("Failed to parse eip info")
		return err
	}

	// Check if EIP exist
	eip, err := k8s.KubeOvnClient.KubeovnV1().IptablesEIPs().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Any("info", s).Msg("Failed to find eip")
		return err
	}

	if err := utils.CheckProjectLabel(eip, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("EIP access denied")
		return err
	}

	if err := utils.IsEditAllowed(eip.GetLabels()); err != nil {
		log.Error().Err(err).Any("info", s).Str("namespace", namespace).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	// Detect old type vs new type
	isFipMode := s.Spec.InternalIP != ""
	isOldModeFip := false
	oldFip, _ := k8s.KubeOvnClient.KubeovnV1().IptablesFIPRules().Get(ctx, name, metav1.GetOptions{})
	if oldFip.Name != "" {
		isOldModeFip = true
	}

	// Mode change
	if isFipMode != isOldModeFip {
		if isFipMode {
			if err := fip.CreateFip(ctx, s.Spec.InternalIP, s.Metadata); err != nil {
				log.Err(err).Any("info", s).Msg("Failed to create FIP, cleaning EIP")
				cleanEip(ctx, s.GetProjectID(), s.GetResourceEffectiveID())
				return err
			}
		} else {
			for _, internalCIDR := range s.Spec.SNAT {
				if err := snat.CreateSNAT(ctx, internalCIDR, s.Metadata); err != nil {
					log.Err(err).Any("info", s).Msg("Failed to create SNAT, cleaning EIP")
					cleanEip(ctx, s.GetProjectID(), s.GetResourceEffectiveID())
					return err
				}
			}

			if s.Spec.DNAT != nil {
				for _, item := range s.Spec.DNAT {
					if err := dnat.CreateDNAT(ctx, item, s.Metadata); err != nil {
						log.Err(err).Any("info", s).Any("dnat", item).Msg("Failed to create DNAT, cleaning EIP")
						cleanEip(ctx, s.GetProjectID(), s.GetResourceEffectiveID())
						return err
					}
				}
			}
		}

		// If Old mode was IP then clear FIP
		if isOldModeFip {
			if err := k8s.KubeOvnClient.KubeovnV1().IptablesFIPRules().Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to delete FIP Rule")
					return err
				}
			}
		} else {
			// Else clear SNAT and DNAT
			snatList, err := snat.ListResources(ctx, namespace, name)
			if err != nil {
				log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to list SNAT Rules")
				return err
			}

			for _, item := range snatList {
				if err := k8s.KubeOvnClient.KubeovnV1().IptablesSnatRules().Delete(ctx, item.Name, metav1.DeleteOptions{}); err != nil {
					if !apierrors.IsNotFound(err) {
						log.Err(err).Str("namespace", namespace).Str("name", name).Str("snat", item.Name).Msg("Failed to delete SNAT Rule")
						return err
					}
				}
			}

			dnatList, err := dnat.List(ctx, namespace, name)
			if err != nil {
				log.Err(err).Msg("failed to retrieve attached DNAT")
				return err
			}

			for _, item := range dnatList {
				if err := k8s.KubeOvnClient.KubeovnV1().IptablesDnatRules().Delete(ctx, item.Name, metav1.DeleteOptions{}); err != nil {
					log.Err(err).Any("dnatList", dnatList).Str("namespace", namespace).Str("name", name).Msg("Failed to delete DNAT")
					return err
				}
			}
		}
	} else {
		// Just update

		if isFipMode {
			// Due to OVN restriction, we can't update the FIP
			// Instead we delete and recreate
			if err := k8s.KubeOvnClient.KubeovnV1().IptablesFIPRules().Delete(ctx, oldFip.Name, metav1.DeleteOptions{}); err != nil {
				log.Err(err).Any("info", s).Msg("Updating FIP - Failed to remove old FIP")
				return err
			}

			// Await for FIP deletion
			time.Sleep(300 * time.Millisecond)

			newFip := v1.IptablesFIPRule{
				ObjectMeta: metav1.ObjectMeta{Name: oldFip.Name, Labels: oldFip.Labels},
				Spec: v1.IptablesFIPRuleSpec{
					EIP:        oldFip.Spec.EIP,
					InternalIP: strings.TrimSpace(s.Spec.InternalIP),
				},
			}

			oldFip.Spec.InternalIP = strings.TrimSpace(s.Spec.InternalIP)
			if _, err := k8s.KubeOvnClient.KubeovnV1().IptablesFIPRules().Create(ctx, &newFip, metav1.CreateOptions{}); err != nil {
				log.Err(err).Any("info", s).Msg("Updating FIP - Failed to create new FIP")
				return err
			}
		} else {
			existingSnats, err := snat.ListResources(ctx, namespace, name)
			if err != nil {
				log.Err(err).Any("info", s).Msg("Failed to list SNAT")
				return err
			}

			if err := snat.UpdateSnat(ctx, s.Metadata, s.Spec.SNAT, existingSnats); err != nil {
				log.Err(err).Msg("failed to update SNAT")
				return err
			}

			dnatList, err := dnat.ListResources(ctx, namespace, name)
			if err != nil {
				log.Err(err).Msg("failed to retrieve attached DNAT")
				return err
			}

			if err := dnat.UpdateDnat(ctx, s.Metadata, s.Spec.DNAT, dnatList); err != nil {
				log.Err(err).Msg("failed to update DNAT")
				return err
			}
		}
	}

	return nil
}
