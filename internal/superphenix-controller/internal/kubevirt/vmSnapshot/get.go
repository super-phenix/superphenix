package vmSnapshot

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetVmSnapshot(ctx context.Context, namespace, name string) (view.VmSnapshotView, error) {
	log := logger.GetLogger(ctx)
	snapshot, err := config.VirtClient.VirtualMachineSnapshot(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error getting vm snapshot")
		return view.VmSnapshotView{}, err
	}
	if err := utils.CheckProjectLabel(snapshot, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("VM snapshot access denied")
		return view.VmSnapshotView{}, err
	}

	return view.VmSnapshotToView(*snapshot), nil
}

func GetVmSnapshotContent(ctx context.Context, namespace, name string) (view.VmSnapshotContentView, error) {
	log := logger.GetLogger(ctx)
	if name == "" {
		return view.VmSnapshotContentView{}, fmt.Errorf("no content name")
	}

	snapshotContent, err := config.VirtClient.VirtualMachineSnapshotContent(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error getting vm snapshot content")
		return view.VmSnapshotContentView{}, err
	}

	snapshotNames := make([]string, 0)
	for _, backup := range snapshotContent.Spec.VolumeBackups {
		snapshotNames = append(snapshotNames, *backup.VolumeSnapshotName)

	}

	return view.VmSnapshotContentView{
		Vm: struct {
			LocalId     string `json:"localId"`
			EffectiveId string `json:"effectiveId"`
		}{
			LocalId:     snapshotContent.Spec.Source.VirtualMachine.Labels[spxId.SpxLabelResourceLocalID],
			EffectiveId: snapshotContent.Spec.Source.VirtualMachine.Labels[spxId.SpxLabelResourceEffectiveID],
		},
		VolumesSnapshot: snapshotNames,
	}, nil
}
