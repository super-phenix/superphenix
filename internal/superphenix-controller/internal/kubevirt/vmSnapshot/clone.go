package vmSnapshot

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/google/uuid"
	v1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/api/snapshot/v1beta1"
)

type CloneVmSnapshotInfo struct {
	Id string `json:"id"`
}

type CloneVmSnapshotDisk struct {
	Id  string `json:"id"`
	Eid string `json:"eid"`
}
type CloneVmSnapshotResponse struct {
	Disks []CloneVmSnapshotDisk `json:"disks"`
}

func (info *CloneVmSnapshotInfo) CloneVmSnapshot(ctx context.Context, orgId, projectId, snapshotEid string) (*CloneVmSnapshotResponse, error) {
	log := logger.GetLogger(ctx)
	namespace := spxId.ToSPXID(projectId)
	snapshot, err := GetVmSnapshot(ctx, namespace, snapshotEid)
	if err != nil {
		log.Err(err).Msgf("Error getting vmSnapshot %s in namespace %s", snapshotEid, namespace)
		return nil, err
	}

	now := strconv.FormatInt(time.Now().UTC().UnixMilli(), 10)
	m := spxId.Metadata{}
	if err := m.GenerateMetadata(projectId, orgId, snapshot.Name+"-"+now); err != nil {
		log.Err(err).Msg("Failed to generate metadata")
		return nil, err
	}

	vmMetadata := spxId.Metadata{}
	if err := vmMetadata.GenerateMetadata(projectId, orgId, info.Id); err != nil {
		log.Err(err).Msg("Failed to generate metadata")
		return nil, err
	}

	// For every snapshotVolumes.includedVolumes, generate a new UUIDv4 and EffectiveID
	disks := make([]CloneVmSnapshotDisk, 0)
	volumeRestoreOverrides := make([]v1beta1.VolumeRestoreOverride, 0)

	var includedVolumes []string
	if snapshot.Status.SnapshotVolumes != nil {
		includedVolumes = snapshot.Status.SnapshotVolumes.IncludedVolumes
	}

	for _, volumeName := range includedVolumes {
		diskLocalId := uuid.New().String()
		diskMetadata := spxId.Metadata{}
		if err := diskMetadata.GenerateMetadata(projectId, orgId, diskLocalId); err != nil {
			log.Err(err).Str("volume", volumeName).Msg("Failed to generate disk metadata")
			return nil, err
		}

		disks = append(disks, CloneVmSnapshotDisk{
			Id:  diskLocalId,
			Eid: diskMetadata.GetResourceEffectiveID(),
		})

		volumeRestoreOverrides = append(volumeRestoreOverrides, v1beta1.VolumeRestoreOverride{
			VolumeName:  volumeName,
			RestoreName: diskMetadata.GetResourceEffectiveID(),
			Labels: map[string]string{
				spxId.SpxLabelGitops:              "false",
				spxId.SpxLabelResourceEffectiveID: diskMetadata.GetResourceEffectiveID(),
				spxId.SpxLabelResourceLocalID:     diskLocalId,
			},
		})
	}

	readinessPolicy := v1beta1.VirtualMachineRestoreStopTarget
	restorePolicy := v1beta1.VolumeRestorePolicyInPlace
	volumeOwnershipPolicy := v1beta1.VolumeOwnershipPolicyNone

	// Patches for VM/VMI labels
	targetEid := vmMetadata.GetResourceEffectiveID()
	targetLocalId := vmMetadata.GetResourceLocalID()
	patches := []string{
		fmt.Sprintf(`{"op": "replace", "path": "/metadata/labels/superphenix.net~1resourceEffectiveID", "value": "%s"}`, targetEid),
		fmt.Sprintf(`{"op": "replace", "path": "/spec/template/metadata/labels/superphenix.net~1resourceEffectiveID", "value": "%s"}`, targetEid),
		fmt.Sprintf(`{"op": "replace", "path": "/metadata/labels/superphenix.net~1resourceLocalID", "value": "%s"}`, targetLocalId),
		fmt.Sprintf(`{"op": "replace", "path": "/spec/template/metadata/labels/superphenix.net~1resourceLocalID", "value": "%s"}`, targetLocalId),
		`{"op": "replace", "path": "/metadata/labels/superphenix.net~1gitops", "value": "false"}`,
		`{"op": "replace", "path": "/spec/template/metadata/labels/superphenix.net~1gitops", "value": "false"}`,
	}

	apiGroup := "kubevirt.io"
	vmRestore := v1beta1.VirtualMachineRestore{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:   m.GetResourceEffectiveID(),
			Labels: m.GetLabels(),
		},
		Spec: v1beta1.VirtualMachineRestoreSpec{
			VirtualMachineSnapshotName: snapshotEid,
			TargetReadinessPolicy:      &readinessPolicy,
			VolumeRestorePolicy:        &restorePolicy,
			VolumeOwnershipPolicy:      &volumeOwnershipPolicy,
			Target: v1.TypedLocalObjectReference{
				APIGroup: &apiGroup,
				Kind:     "VirtualMachine",
				Name:     targetEid,
			},
			VolumeRestoreOverrides: volumeRestoreOverrides,
			Patches:                patches,
		},
	}

	_, err = config.VirtClient.VirtualMachineRestore(namespace).Create(ctx, &vmRestore, k8smetav1.CreateOptions{})
	if err != nil {
		log.Err(err).Msgf("Error cloning vmSnapshot %s in namespace %s", snapshotEid, namespace)
		return nil, err
	}

	return &CloneVmSnapshotResponse{
		Disks: disks,
	}, nil
}
