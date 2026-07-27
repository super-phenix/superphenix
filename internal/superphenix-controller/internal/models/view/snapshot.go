package view

import (
	"encoding/json"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	snapschedulerv1 "github.com/backube/snapscheduler/api/v1"
	v1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/rs/zerolog/log"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type SnapshotView struct {
	k8smetav1.TypeMeta `json:",inline"`
	ObjectMeta         `json:"metadata,omitempty"`
	Spec               VolumeSnapshotSpec      `json:"spec"`
	Status             v1.VolumeSnapshotStatus `json:"status,omitempty"`
}

// VolumeSnapshotSpec describes the common attributes of a volume snapshot.
type VolumeSnapshotSpec struct {
	// source specifies where a snapshot will be created from.
	// This field is immutable after creation.
	// Required.
	Source v1.VolumeSnapshotSource `json:"source" protobuf:"bytes,1,opt,name=source"`

	// VolumeSnapshotClassName is the name of the VolumeSnapshotClass
	// requested by the VolumeSnapshot.
	// VolumeSnapshotClassName may be left nil to indicate that the default
	// SnapshotClass should be used.
	// A given cluster may have multiple default Volume SnapshotClasses: one
	// default per CSI Driver. If a VolumeSnapshot does not specify a SnapshotClass,
	// VolumeSnapshotSource will be checked to figure out what the associated
	// CSI Driver is, and the default VolumeSnapshotClass associated with that
	// CSI Driver will be used. If more than one VolumeSnapshotClass exist for
	// a given CSI Driver and more than one have been marked as default,
	// CreateSnapshot will fail and generate an event.
	// Empty string is not allowed for this field.
	// +optional
	VolumeSnapshotClassName *string `json:"volumeSnapshotClassName,omitempty" protobuf:"bytes,2,opt,name=volumeSnapshotClassName"`
}
type SnapshotScheduleView struct {
	k8smetav1.TypeMeta `json:",inline"`
	ObjectMeta         `json:"metadata,omitempty"`
	Spec               snapschedulerv1.SnapshotScheduleSpec   `json:"spec"`
	Status             snapschedulerv1.SnapshotScheduleStatus `json:"status,omitempty"`
	Children           []SnapshotView                         `json:"children,omitempty"`
	ChildrenCount      int                                    `json:"childrenCount"`
}

func UnstructuredSnapshotToView(snapshot *unstructured.Unstructured) SnapshotView {
	var view SnapshotView
	err := utils.UnstructuredToStruct(snapshot, &view)
	if err != nil {
		log.Error().AnErr("error converting snapshot to view", err).Any("snapshot", snapshot).Send()
		return SnapshotView{}
	}
	view.Labels = utils.FilterLabels(view.Labels)
	view.Annotations = utils.FilterAnnotations(view.Annotations)
	if view.Spec.VolumeSnapshotClassName != nil {
		snapshotClassName := convertStorageClassName(*view.Spec.VolumeSnapshotClassName)
		view.Spec.VolumeSnapshotClassName = &snapshotClassName
	}
	return view
}

func SnapshotToView(snapshot v1.VolumeSnapshot) SnapshotView {
	snapshot.Labels = utils.FilterLabels(snapshot.GetLabels())
	snapshot.Annotations = utils.FilterAnnotations(snapshot.GetAnnotations())

	blockStr, err := json.Marshal(snapshot)
	if err != nil {
		log.Error().AnErr("error converting snapshot to view", err).Any("snapshot", snapshot).Send()
		return SnapshotView{}
	}
	var view SnapshotView
	err = json.Unmarshal(blockStr, &view)
	if err != nil {
		log.Error().AnErr("error converting snapshot to view", err).Any("snapshot", snapshot).Send()
		return SnapshotView{}
	}
	if view.Spec.VolumeSnapshotClassName != nil {
		snapshotClassName := convertStorageClassName(*view.Spec.VolumeSnapshotClassName)
		view.Spec.VolumeSnapshotClassName = &snapshotClassName
	}
	return view
}

func UnstructuredToSnapshotScheduleView(obj *unstructured.Unstructured) SnapshotScheduleView {
	var view SnapshotScheduleView
	err := utils.UnstructuredToStruct(obj, &view)
	if err != nil {
		log.Error().AnErr("error converting snapshot schedule to view", err).Any("object", obj).Send()
		return SnapshotScheduleView{}
	}
	view.Labels = utils.FilterLabels(view.Labels)
	view.Annotations = utils.FilterAnnotations(view.Annotations)
	return view
}

func SnapshotViewToResource(snapshotView SnapshotView) Snapshot {
	return Snapshot{
		Resource: Resource{
			ID:          snapshotView.Labels[spxId.SpxLabelResourceLocalID],
			EId:         snapshotView.Name,
			ProductName: snapshotView.Labels[spxId.SpxLabelResourceName],
			Gitops:      snapshotView.Labels[spxId.SpxLabelGitops],
		},
		Snapshot: &snapshotView,
	}
}

func SnapshotScheduleViewToResource(scheduleView SnapshotScheduleView) Snapshot {
	return Snapshot{
		Resource: Resource{
			ID:          scheduleView.Labels[spxId.SpxLabelResourceLocalID],
			EId:         scheduleView.Name,
			ProductName: scheduleView.Labels[spxId.SpxLabelResourceName],
			Gitops:      scheduleView.Labels[spxId.SpxLabelGitops],
		},
		SnapshotSchedule: &scheduleView,
	}
}
