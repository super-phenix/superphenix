package vmSnapshot

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/api/snapshot/v1beta1"
)

// ErrRestoreSourceMissing is returned when the snapshot source VM does not exist
var ErrRestoreSourceMissing = errors.New("snapshot source instance no longer exists")

// RestoreVmSnapshot restores the snapshot `name` in place onto its source VM, which must exist
func RestoreVmSnapshot(ctx context.Context, orgId, projectId, name string) error {
	log := logger.GetLogger(ctx)
	namespace := spxId.ToSPXID(projectId)
	snapshot, err := GetVmSnapshot(ctx, namespace, name)
	if err != nil {
		log.Err(err).Msgf("Error getting vmSnapshot %s in namespace %s", name, namespace)
		return err
	}

	sourceVm, err := config.VirtClient.VirtualMachine(namespace).Get(ctx, snapshot.Spec.Source.Name, k8smetav1.GetOptions{})
	if err == nil {
		err = utils.CheckProjectLabel(sourceVm, namespace)
	}
	if apierrors.IsNotFound(err) {
		log.Warn().Str("namespace", namespace).Str("source", snapshot.Spec.Source.Name).Msg("Snapshot source VM not found")
		return ErrRestoreSourceMissing
	}
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("source", snapshot.Spec.Source.Name).Msg("Error getting snapshot source VM")
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
