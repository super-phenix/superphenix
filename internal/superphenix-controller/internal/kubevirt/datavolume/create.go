package datavolume

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/k8s/pvc"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubevirt/volumeSnapshot"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const storageFormat = "%sGi"

func (info *CreateDiskInfo) CreateDisk(ctx context.Context, namespace string) error {
	log := logger.GetLogger(ctx)
	metadata := spxId.Metadata{}
	err := metadata.ConvertToSpxMetadata(info.Metadata)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Any("info", info).Msg("Failed to parse datavolume informations")
		return err
	}

	minStorageSize := resource.MustParse(fmt.Sprintf(storageFormat, "1"))
	source := v1beta1.DataVolumeSource{}
	switch srcType := info.General.Source.Type; srcType {
	case SourceTypeRegistry:
		url := strings.TrimSpace(info.General.Source.URL)
		source.Registry = &v1beta1.DataVolumeSourceRegistry{
			URL: &url,
		}
	case SourceTypeHttp:
		url := strings.TrimSpace(info.General.Source.URL)
		source.HTTP = &v1beta1.DataVolumeSourceHTTP{
			URL: url,
		}

	case SourceTypeClone:
		source.PVC = &v1beta1.DataVolumeSourcePVC{
			Namespace: namespace,
			Name:      info.General.Source.Clone,
		}

		disk, err := pvc.GetPVC(ctx, namespace, info.General.Source.Clone)
		if err != nil {
			log.Err(err).Str("namespace", namespace).Any("info", info).Msg("Failed to get disk source")
			return fmt.Errorf("failed to get disk source")
		}

		if disk.Spec.Resources.Requests.Storage() != nil {
			minStorageSize = *disk.Spec.Resources.Requests.Storage()
		}

	case SourceTypeSnapshot:
		source.Snapshot = &v1beta1.DataVolumeSourceSnapshot{
			Namespace: namespace,
			Name:      info.General.Source.Snapshot,
		}

		snapshotResource, err := volumeSnapshot.GetSnapshot(ctx, namespace, info.General.Source.Snapshot)
		if err != nil {
			log.Err(err).Str("namespace", namespace).Any("info", info).Msg("Failed to get snapshot source")
			return fmt.Errorf("failed to get snapshot source")
		}

		if snapshotResource.Snapshot != nil && snapshotResource.Snapshot.Status.RestoreSize != nil {
			minStorageSize = *snapshotResource.Snapshot.Status.RestoreSize
		}

	case SourceTypeBlank:
		source.Blank = &v1beta1.DataVolumeBlankImage{}
	}
	storage := resource.MustParse(fmt.Sprintf(storageFormat, info.General.Storage))

	// 0 if the quantity is equal to y,
	// -1 if the quantity is less than y, or
	// 1 if the quantity is greater than y.
	if storage.Cmp(minStorageSize) < 0 {
		log.Error().Str("namespace", namespace).Any("info", info).Str("minStorageSize", minStorageSize.String()).Msg("Storage size is incorrect")
		return fmt.Errorf("storage size is incorrect")
	}

	volumeMode := v1.PersistentVolumeBlock
	storageClassName := ""
	if fullname, ok := config.Global.StorageClassMapping[info.General.StorageClass]; ok {
		storageClassName = fullname
	}

	if storageClassName == "" {
		err := fmt.Errorf("no storage class found")
		log.Err(err).Str("namespace", namespace).Any("info", info).Msg("Failed to create disk")
		return err
	}

	defaultAnnotations := make(map[string]string)
	for _, annotation := range config.Global.Datavolume.DefaultAnnotations {
		defaultAnnotations[annotation.Key] = annotation.Value
	}

	labels := metadata.GetLabels()
	labels["superphenix.net/ignoreNetworkPolicies"] = "true"
	labels["superphenix.net/workloadClass"] = "datavolume-importer"

	customLabels, err := utils.ParseLabels(info.General.Labels, utils.CustomLabelPrefix)
	if err != nil {
		return fmt.Errorf("error parsing custom labels: %w", err)
	}
	if len(customLabels) > utils.MaxCustomLabelNumber {
		return fmt.Errorf("too many custom labels %d > %d", len(customLabels), utils.MaxCustomLabelNumber)
	}

	// Add custom labels
	maps.Copy(labels, customLabels)

	_, err = config.VirtClient.CdiClient().CdiV1beta1().DataVolumes(namespace).Create(ctx, &v1beta1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        metadata.GetResourceEffectiveID(),
			Labels:      labels,
			Annotations: defaultAnnotations,
		},
		Spec: v1beta1.DataVolumeSpec{
			Source: &source,
			Storage: &v1beta1.StorageSpec{
				VolumeMode:       &volumeMode,
				AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
				StorageClassName: &storageClassName,
				Resources: v1.VolumeResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceStorage: storage,
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Failed to create datavolume")
	}
	return err
}
