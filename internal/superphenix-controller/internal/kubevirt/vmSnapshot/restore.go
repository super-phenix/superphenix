package vmSnapshot

import (
	"context"
	"strconv"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/api/snapshot/v1beta1"
)

// checkSourceLocalId returns a BadRequest error unless localId is the local
// ID of the snapshot source VM, i.e. its effective ID is the source name.
func checkSourceLocalId(orgId, projectId, localId, sourceName string) error {
	if localId == "" {
		return apierrors.NewBadRequest("localId is required")
	}

	m := spxId.Metadata{}
	if err := m.GenerateMetadata(projectId, orgId, localId); err != nil {
		return apierrors.NewBadRequest("localId is invalid")
	}

	if m.GetResourceEffectiveID() != sourceName {
		return apierrors.NewBadRequest("localId does not match the snapshot source VM")
	}

	return nil
}

// RestoreVmSnapshot restores, in place, the VM the snapshot was taken from.
// localId is the local ID of that source VM, as known by the API: it must
// derive to the snapshot source name, otherwise the API would register the
// restored VM under an arbitrary product ID.
func RestoreVmSnapshot(ctx context.Context, orgId, projectId, name, localId string) error {
	log := logger.GetLogger(ctx)
	namespace := spxId.ToSPXID(projectId)
	snapshot, err := GetVmSnapshot(ctx, namespace, name)
	if err != nil {
		log.Err(err).Msgf("Error getting vmSnapshot %s in namespace %s", name, namespace)
		return err
	}

	if err := checkSourceLocalId(orgId, projectId, localId, snapshot.Spec.Source.Name); err != nil {
		log.Warn().Err(err).Str("snapshot", name).Str("localId", localId).Str("source", snapshot.Spec.Source.Name).Msg("Restore refused: localId does not match the snapshot source VM")
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
