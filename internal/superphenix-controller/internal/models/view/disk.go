package view

import (
	"encoding/json"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

type DiskView struct {
	k8smetav1.TypeMeta `json:",inline"`
	ObjectMeta         `json:"metadata,omitempty"`
	Spec               DataVolumeSpec   `json:"spec"`
	Status             DataVolumeStatus `json:"status,omitempty"`
}

type MountStatus struct {
	IsMounted bool   `json:"isMounted"`
	By        string `json:"by"`
}

// DataVolumeSpec defines the DataVolume type specification
type DataVolumeSpec struct {
	//Source is the src of the data for the requested DataVolume
	// +optional
	Source *v1beta1.DataVolumeSource `json:"source,omitempty"`
	// Storage is the requested storage specification
	Storage StorageSpec `json:"storage,omitempty"`
	// StorageClassName is the name of the StorageClass required by the DataVolume.
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// StorageSpec defines the Storage type specification
type StorageSpec struct {
	// AccessModes contains the desired access modes the volume should have.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
	// Resources represents the minimum resources the volume should have.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources
	// +optional
	Resources corev1.VolumeResourceRequirements `json:"resources,omitempty"`
	// volumeMode defines what type of volume is required by the claim.
	// Value of Filesystem is implied when not included in claim spec.
	// +optional
	VolumeMode *corev1.PersistentVolumeMode `json:"volumeMode,omitempty"`
}

// DataVolumeStatus contains the current status of the DataVolume
type DataVolumeStatus struct {
	// ClaimName is the name of the underlying PVC used by the DataVolume.
	ClaimName string `json:"claimName,omitempty"`
	//Phase is the current phase of the data volume
	Phase      v1beta1.DataVolumePhase       `json:"phase,omitempty"`
	Progress   v1beta1.DataVolumeProgress    `json:"progress,omitempty"`
	Conditions []v1beta1.DataVolumeCondition `json:"conditions,omitempty" optional:"true"`
}

func UnstructuredDiskToView(disk *unstructured.Unstructured) DiskView {
	var view DiskView
	if err := utils.UnstructuredToStruct(disk, &view); err != nil {
		log.Error().AnErr("error converting disk to view", err).Any("disk", disk).Send()
		return DiskView{}
	}
	view.Labels = utils.FilterLabels(view.Labels)
	view.Annotations = utils.FilterAnnotations(view.Annotations)
	if view.Spec.StorageClassName != nil {
		storageClassName := convertStorageClassName(*view.Spec.StorageClassName)
		view.Spec.StorageClassName = &storageClassName
	}
	return view
}

func DiskToView(disk v1beta1.DataVolume) DiskView {
	disk.Labels = utils.FilterLabels(disk.GetLabels())
	disk.Annotations = utils.FilterAnnotations(disk.GetAnnotations())

	diskStr, err := json.Marshal(disk)
	if err != nil {
		log.Error().AnErr("error converting disk to view", err).Any("disk", disk).Send()
		return DiskView{}
	}
	var view DiskView
	err = json.Unmarshal(diskStr, &view)
	if err != nil {
		log.Error().AnErr("error converting disk to view", err).Any("disk", disk).Send()
		return DiskView{}
	}
	if view.Spec.StorageClassName != nil {
		storageClassName := convertStorageClassName(*view.Spec.StorageClassName)
		view.Spec.StorageClassName = &storageClassName
	}
	return view
}

func DiskViewToResource(diskView DiskView, pvcView PVCView) Disk {
	return Disk{
		Resource: Resource{
			ID:          pvcView.Labels[spxId.SpxLabelResourceLocalID],
			EId:         pvcView.Name,
			ProductName: pvcView.Labels[spxId.SpxLabelResourceName],
			Gitops:      pvcView.Labels[spxId.SpxLabelGitops],
		},
		Disk: diskView,
		PVC:  pvcView,
	}
}

type PVCView struct {
	k8smetav1.TypeMeta `json:",inline"`
	ObjectMeta         `json:"metadata,omitempty"`
	Spec               PVCSpec   `json:"spec"`
	Status             PVCStatus `json:"status,omitempty"`
}

// PVCSpec defines the PVC type specification
type PVCSpec struct {
	// accessModes contains the desired access modes the volume should have.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1
	// +optional
	// +listType=atomic
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty" protobuf:"bytes,1,rep,name=accessModes,casttype=PersistentVolumeAccessMode"`
	// resources represents the minimum resources the volume should have.
	// If RecoverVolumeExpansionFailure feature is enabled users are allowed to specify resource requirements
	// that are lower than previous value but must still be higher than capacity recorded in the
	// status field of the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources
	// +optional
	Resources corev1.VolumeResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
	// volumeName is the binding reference to the PersistentVolume backing this claim.
	// +optional
	VolumeName string `json:"volumeName,omitempty" protobuf:"bytes,3,opt,name=volumeName"`
	// storageClassName is the name of the StorageClass required by the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty" protobuf:"bytes,5,opt,name=storageClassName"`
	// volumeMode defines what type of volume is required by the claim.
	// Value of Filesystem is implied when not included in claim spec.
	// +optional
	VolumeMode *corev1.PersistentVolumeMode `json:"volumeMode,omitempty" protobuf:"bytes,6,opt,name=volumeMode,casttype=PersistentVolumeMode"`
	// dataSource field can be used to specify either:
	// * An existing VolumeSnapshot object (snapshot.storage.k8s.io/VolumeSnapshot)
	// * An existing PVC (PersistentVolumeClaim)
	// If the provisioner or an external controller can support the specified data source,
	// it will create a new volume based on the contents of the specified data source.
	// When the AnyVolumeDataSource feature gate is enabled, dataSource contents will be copied to dataSourceRef,
	// and dataSourceRef contents will be copied to dataSource when dataSourceRef.namespace is not specified.
	// If the namespace is specified, then dataSourceRef will not be copied to dataSource.
	// +optional
	DataSource *corev1.TypedLocalObjectReference `json:"dataSource,omitempty" protobuf:"bytes,7,opt,name=dataSource"`
}

// PVCStatus contains the current status of the PVC
type PVCStatus struct {
	// phase represents the current phase of PersistentVolumeClaim.
	// +optional
	Phase corev1.PersistentVolumeClaimPhase `json:"phase,omitempty" protobuf:"bytes,1,opt,name=phase,casttype=PersistentVolumeClaimPhase"`
	// accessModes contains the actual access modes the volume backing the PVC has.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1
	// +optional
	// +listType=atomic
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty" protobuf:"bytes,2,rep,name=accessModes,casttype=PersistentVolumeAccessMode"`
	// capacity represents the actual resources of the underlying volume.
	// +optional
	Capacity corev1.ResourceList `json:"capacity,omitempty" protobuf:"bytes,3,rep,name=capacity,casttype=ResourceList,castkey=ResourceName"`
}

func UnstructuredPVCToView(pvc *unstructured.Unstructured) PVCView {
	var view PVCView
	if err := utils.UnstructuredToStruct(pvc, &view); err != nil {
		log.Error().AnErr("error converting pvc to view", err).Any("pvc", pvc).Send()
		return PVCView{}
	}
	view.Labels = utils.FilterLabels(view.Labels)
	view.Annotations = utils.FilterAnnotations(view.Annotations)
	if view.Spec.StorageClassName != nil {
		storageClassName := convertStorageClassName(*view.Spec.StorageClassName)
		view.Spec.StorageClassName = &storageClassName
	}
	return view
}

func PVCToView(pvc corev1.PersistentVolumeClaim) PVCView {
	pvc.Labels = utils.FilterLabels(pvc.GetLabels())
	pvc.Annotations = utils.FilterAnnotations(pvc.GetAnnotations())

	diskStr, err := json.Marshal(pvc)
	if err != nil {
		log.Error().AnErr("error converting pvc to view", err).Any("pvc", pvc).Send()
		return PVCView{}
	}
	var view PVCView
	err = json.Unmarshal(diskStr, &view)
	if err != nil {
		log.Error().AnErr("error converting pvc to view", err).Any("pvc", pvc).Send()
		return PVCView{}
	}
	if view.Spec.StorageClassName != nil {
		storageClassName := convertStorageClassName(*view.Spec.StorageClassName)
		view.Spec.StorageClassName = &storageClassName
	}
	return view
}
