package pvc

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UpdateDiskInfo struct {
	General struct {
		Storage string   `json:"storage"` // Storage Size
		Labels  []string `json:"labels,omitempty"`
	} `json:"general"`
}

const storageFormat = "%sGi"

func (info *UpdateDiskInfo) UpdatePVC(ctx context.Context, namespace, name string, force bool) error {
	log := logger.GetLogger(ctx)
	pvcToUpdate, err := config.K8sClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Any("info", info).Msg("Failed to get PVC")
		return fmt.Errorf("failed to get disk")
	}

	if err := utils.CheckProjectLabel(pvcToUpdate, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("PVC access denied")
		return err
	}

	isEditAllowed := utils.IsEditAllowed
	if force {
		isEditAllowed = utils.IsEditAllowedForceGitops
	}
	if err := isEditAllowed(pvcToUpdate.GetLabels()); err != nil {
		log.Error().Err(err).Any("info", info).Str("namespace", namespace).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}
	if force && pvcToUpdate.GetLabels()[spxId.SpxLabelGitops] == "true" {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("Gitops guard bypassed with force for disk update")
	}

	oldStorage := pvcToUpdate.Status.Capacity.Storage()
	newStorage := resource.MustParse(fmt.Sprintf(storageFormat, info.General.Storage))

	// Old is greater than new
	if oldStorage.Cmp(newStorage) == 1 {
		log.Error().Str("old", oldStorage.String()).Str("new", newStorage.String()).Msg("Cannot set a lower storage value")
		return fmt.Errorf("cannot set a lower storage value")
	}

	pvcToUpdate.Spec.Resources.Requests[v1.ResourceStorage] = newStorage

	// Handle custom labels
	customLabels, err := utils.ParseLabels(info.General.Labels, utils.CustomLabelPrefix)
	if err != nil {
		log.Error().Err(err).Any("info", info).Str("namespace", namespace).Str("name", name).Msg("Cannot parse custom labels")
		return fmt.Errorf("error parsing custom labels: %w", err)
	}
	if len(customLabels) > utils.MaxCustomLabelNumber {
		log.Error().Any("info", info).Str("namespace", namespace).Str("name", name).Msgf("Too many custom labels %d > %d", len(customLabels), utils.MaxCustomLabelNumber)
		return fmt.Errorf("too many custom labels %d > %d", len(customLabels), utils.MaxCustomLabelNumber)
	}

	labels := pvcToUpdate.GetLabels()
	// Remove old custom labels
	maps.DeleteFunc(labels, func(k string, v string) bool {
		return strings.HasPrefix(k, utils.CustomLabelPrefix)
	})
	// Add custom labels
	maps.Copy(labels, customLabels)
	pvcToUpdate.Labels = labels

	if _, err := config.K8sClient.CoreV1().PersistentVolumeClaims(namespace).Update(ctx, pvcToUpdate, metav1.UpdateOptions{}); err != nil {
		log.Err(err).Any("pvc", pvcToUpdate).Msg("Failed to update disk")
		return fmt.Errorf("failed to update disk")
	}

	return nil
}
