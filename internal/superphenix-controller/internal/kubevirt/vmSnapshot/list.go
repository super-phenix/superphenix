package vmSnapshot

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListVmSnapshots(ctx context.Context, namespace string) []view.VmSnapshotView {
	log := logger.GetLogger(ctx)
	if namespace == "" {
		log.Error().Msg("No namespace provided")
		return make([]view.VmSnapshotView, 0)
	}

	list, err := informers.WatcherSet[informers.VirtualMachineSnapshot].ByIndex("namespace", namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error listing snapshots")
		return make([]view.VmSnapshotView, 0)
	}

	vmSnapshots := make([]view.VmSnapshotView, 0)
	for _, item := range list {
		vmSnapshot := item.(*unstructured.Unstructured)
		vmSnapshotView := view.UnstructuredVmSnapshotToView(vmSnapshot)
		vmSnapshots = append(vmSnapshots, vmSnapshotView)
	}

	return vmSnapshots
}

func ListVmSnapshotContents(ctx context.Context, namespace string) map[string]view.VmSnapshotContentView {
	log := logger.GetLogger(ctx)
	contents := make(map[string]view.VmSnapshotContentView)

	if namespace == "" {
		log.Error().Msg("No namespace provided")
		return contents
	}

	contentList, err := config.VirtClient.VirtualMachineSnapshotContent(namespace).List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error listing vm snapshot contents")
		return contents
	}

	for _, content := range contentList.Items {
		snapshotNames := make([]string, 0)
		for _, backup := range content.Spec.VolumeBackups {
			snapshotNames = append(snapshotNames, *backup.VolumeSnapshotName)
		}

		contents[content.Name] = view.VmSnapshotContentView{
			Vm: struct {
				LocalId     string `json:"localId"`
				EffectiveId string `json:"effectiveId"`
			}{
				LocalId:     content.Spec.Source.VirtualMachine.Labels[spxId.SpxLabelResourceLocalID],
				EffectiveId: content.Spec.Source.VirtualMachine.Labels[spxId.SpxLabelResourceEffectiveID],
			},
			VolumesSnapshot: snapshotNames,
		}
	}

	return contents
}
