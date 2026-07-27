package vmSnapshot

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/api/snapshot/v1beta1"
)

type CreateVmSnapshotInfo struct {
	spxId.Metadata
	General struct {
		Source string `json:"source"`
	} `json:"general"`
}

func (info *CreateVmSnapshotInfo) CreateVmSnapshot(ctx context.Context, namespace string) error {
	log := logger.GetLogger(ctx)
	metadata := spxId.Metadata{}
	if err := metadata.ConvertToSpxMetadata(info.Metadata); err != nil {
		log.Err(err).Str("namespace", namespace).Any("info", info).Msg("Failed to parse vm snapshot info")
		return err
	}

	apiGroup := "kubevirt.io"
	_, err := config.VirtClient.VirtualMachineSnapshot(namespace).Create(ctx, &v1beta1.VirtualMachineSnapshot{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:   metadata.GetResourceEffectiveID(),
			Labels: metadata.GetLabels(),
		},
		Spec: v1beta1.VirtualMachineSnapshotSpec{
			Source: v1.TypedLocalObjectReference{
				APIGroup: &apiGroup,
				Kind:     "VirtualMachine",
				Name:     info.General.Source,
			},
		},
	}, k8smetav1.CreateOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Failed to create vm snapshot")
		return err
	}

	return nil
}
