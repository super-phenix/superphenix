package dnat

import (
	"context"
	"fmt"

	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UpdateDnat(ctx context.Context, eipMetadata spxId.Metadata, newDNATS []InfoDNAT, oldDNATs []v1.IptablesDnatRule) error {
	log := logger.GetLogger(ctx)
	m := map[string]struct {
		Old bool
		New bool
		EID string
		InfoDNAT
	}{}

	for _, s1Val := range oldDNATs {
		val := fmt.Sprintf("%s-%s-%s-%s", s1Val.Spec.ExternalPort, s1Val.Spec.InternalIP, s1Val.Spec.InternalPort, s1Val.Spec.Protocol)
		m[val] = struct {
			Old bool
			New bool
			EID string
			InfoDNAT
		}{Old: true, EID: s1Val.Name}
	}
	for _, s2Val := range newDNATS {
		val := fmt.Sprintf("%s-%s-%s-%s", s2Val.ExternalPort, s2Val.InternalIP, s2Val.InternalPort, s2Val.Protocol)
		m[val] = struct {
			Old bool
			New bool
			EID string
			InfoDNAT
		}{New: true, Old: m[val].Old, EID: m[val].EID, InfoDNAT: s2Val}
	}

	toDelete := make([]string, 0)

	for _, mVal := range m {
		// old only
		if mVal.Old == true && mVal.New == false {
			toDelete = append(toDelete, mVal.EID)
		}
		// new only
		if !mVal.Old && mVal.New {
			infoDNAT := InfoDNAT{
				ExternalPort: mVal.ExternalPort,
				InternalIP:   mVal.InternalIP,
				InternalPort: mVal.InternalPort,
				Protocol:     mVal.Protocol,
			}

			if err := CreateDNAT(ctx, infoDNAT, eipMetadata); err != nil {
				log.Err(err).Any("dnat", mVal.InfoDNAT).Msg("Failed to create DNAT")
				return err
			}
		}
	}

	for _, eid := range toDelete {
		if err := k8s.KubeOvnClient.KubeovnV1().IptablesDnatRules().Delete(ctx, eid, metav1.DeleteOptions{}); err != nil {
			log.Err(err).Any("dnatList", toDelete).Str("name", eid).Msg("Failed to delete DNAT")
			return err
		}
	}

	return nil
}
