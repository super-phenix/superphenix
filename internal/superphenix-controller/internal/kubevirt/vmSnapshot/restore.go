package vmSnapshot

import (
	"context"
	"strconv"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/api/snapshot/v1beta1"
)

func RestoreVmSnapshot(ctx context.Context, orgId, projectId, name string) error {
	log := logger.GetLogger(ctx)
	namespace := spxId.ToSPXID(projectId)
	snapshot, err := GetVmSnapshot(ctx, namespace, name)
	if err != nil {
		log.Err(err).Msgf("Error getting vmSnapshot %s in namespace %s", name, namespace)
		return err
	}

	now := strconv.FormatInt(time.Now().UTC().UnixMilli(), 10)
	m := spxId.Metadata{}

	if err := m.GenerateMetadata(projectId, orgId, name+"-"+now); err != nil {
		log.Err(err).Msg("Failed to generate metadata")
		return err
	}

	readinessPolicy := v1beta1.VirtualMachineRestoreStopTarget
	restorePolicy := v1beta1.VolumeRestorePolicyInPlace
	volumeOwnershipPolicy := v1beta1.VolumeOwnershipPolicyNone

	_, err = config.VirtClient.VirtualMachineRestore(namespace).Create(ctx, &v1beta1.VirtualMachineRestore{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:   m.GetResourceEffectiveID(),
			Labels: m.GetLabels(),
		},
		Spec: v1beta1.VirtualMachineRestoreSpec{
			Target: v1.TypedLocalObjectReference{
				APIGroup: snapshot.Spec.Source.APIGroup,
				Kind:     snapshot.Spec.Source.Kind,
				Name:     snapshot.Spec.Source.Name,
			},
			VirtualMachineSnapshotName: name,
			TargetReadinessPolicy:      &readinessPolicy,
			VolumeRestorePolicy:        &restorePolicy,
			VolumeOwnershipPolicy:      &volumeOwnershipPolicy,
		},
	}, k8smetav1.CreateOptions{})
	if err != nil {
		log.Err(err).Msgf("Error restoring vmSnapshot %s in namespace %s", name, namespace)
		return err
	}

	return nil
}
