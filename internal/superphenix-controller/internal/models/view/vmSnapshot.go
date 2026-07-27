package view

import (
	"encoding/json"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/rs/zerolog/log"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"kubevirt.io/api/snapshot/v1beta1"
)

type VmSnapshotView struct {
	k8smetav1.TypeMeta `json:",inline"`
	ObjectMeta         `json:"metadata,omitempty"`
	Spec               v1beta1.VirtualMachineSnapshotSpec   `json:"spec"`
	Status             v1beta1.VirtualMachineSnapshotStatus `json:"status,omitempty"`
}

type VmSnapshotContentView struct {
	Vm struct {
		LocalId     string `json:"localId"`
		EffectiveId string `json:"effectiveId"`
	} `json:"vm"`
	VolumesSnapshot []string `json:"volumesSnapshot"`
}

func UnstructuredVmSnapshotToView(snapshot *unstructured.Unstructured) VmSnapshotView {
	var view VmSnapshotView
	err := utils.UnstructuredToStruct(snapshot, &view)
	if err != nil {
		log.Error().AnErr("error converting snapshot to view", err).Any("snapshot", snapshot).Send()
		return VmSnapshotView{}
	}
	view.Labels = utils.FilterLabels(view.Labels)
	view.Annotations = utils.FilterAnnotations(view.Annotations)
	return view
}

func VmSnapshotToView(snapshot v1beta1.VirtualMachineSnapshot) VmSnapshotView {
	snapshot.Labels = utils.FilterLabels(snapshot.GetLabels())
	snapshot.Annotations = utils.FilterAnnotations(snapshot.GetAnnotations())

	blockStr, err := json.Marshal(snapshot)
	if err != nil {
		log.Error().AnErr("error converting snapshot to view", err).Any("snapshot", snapshot).Send()
		return VmSnapshotView{}
	}
	var view VmSnapshotView
	err = json.Unmarshal(blockStr, &view)
	if err != nil {
		log.Error().AnErr("error converting snapshot to view", err).Any("snapshot", snapshot).Send()
		return VmSnapshotView{}
	}
	return view
}

func VmSnapshotViewToResource(snapshotView VmSnapshotView, snapshotContent VmSnapshotContentView) VmSnapshot {
	return VmSnapshot{
		Resource: Resource{
			ID:          snapshotView.Labels[spxId.SpxLabelResourceLocalID],
			EId:         snapshotView.Name,
			ProductName: snapshotView.Labels[spxId.SpxLabelResourceName],
			Gitops:      snapshotView.Labels[spxId.SpxLabelGitops],
		},
		VmSnapshot:        snapshotView,
		VmSnapshotContent: snapshotContent,
	}
}
