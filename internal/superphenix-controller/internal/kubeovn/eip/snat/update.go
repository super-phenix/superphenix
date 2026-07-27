package snat

import (
	"context"
	"strings"

	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/google/uuid"
	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UpdateSnat(ctx context.Context, eipMetadata spxId.Metadata, newSNATs []string, existingSnats []v1.IptablesSnatRule) error {
	log := logger.GetLogger(ctx)
	newSnatsMap := make(map[string]string)
	eipEidUUID, err := uuid.Parse(eipMetadata.ResourceEffectiveId)
	if err != nil {
		log.Err(err).Msg("Failed to parse eip uuid")
		return err
	}

	for _, cidr := range newSNATs {
		cidr = strings.TrimSpace(cidr)
		localId := uuid.NewSHA1(eipEidUUID, []byte(cidr))
		snatMeta := spxId.Metadata{}
		err = snatMeta.GenerateMetadata(eipMetadata.ProjectId, eipMetadata.OrgId, localId.String())
		if err != nil {
			log.Err(err).Str("cidr", cidr).Msg("Failed to generate SNAT metadata")
			return err
		}
		newSnatsMap[snatMeta.GetResourceEffectiveID()] = cidr
	}

	// Delete SNATs that are no longer in the list
	for _, oldSnat := range existingSnats {
		if _, ok := newSnatsMap[oldSnat.Name]; !ok {
			if err := k8s.KubeOvnClient.KubeovnV1().IptablesSnatRules().Delete(ctx, oldSnat.Name, metav1.DeleteOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					log.Err(err).Str("snat", oldSnat.Name).Msg("Failed to delete old SNAT")
					return err
				}
			}
		}
	}

	// Create or Update SNATs
	for snatName, cidr := range newSnatsMap {
		found := false
		for _, oldSnat := range existingSnats {
			if oldSnat.Name == snatName {
				found = true
				break
			}
		}

		if !found {
			snatMeta := spxId.Metadata{}
			// We need to re-generate metadata to get labels
			// Re-calculating localId from cidr
			localId := uuid.NewSHA1(eipEidUUID, []byte(cidr))
			_ = snatMeta.GenerateMetadata(eipMetadata.ProjectId, eipMetadata.OrgId, localId.String())

			snatRule := v1.IptablesSnatRule{
				ObjectMeta: metav1.ObjectMeta{Name: snatName, Labels: snatMeta.GetLabels()},
				Spec: v1.IptablesSnatRuleSpec{
					EIP:          eipMetadata.GetResourceEffectiveID(),
					InternalCIDR: cidr,
				},
			}
			if _, err := k8s.KubeOvnClient.KubeovnV1().IptablesSnatRules().Create(ctx, &snatRule, metav1.CreateOptions{}); err != nil {
				log.Err(err).Str("snat", snatName).Msg("Failed to create new SNAT")
				return err
			}
		}
	}
	return nil
}
