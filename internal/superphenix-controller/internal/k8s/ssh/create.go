package ssh

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreateSSHInfo struct {
	spxId.Metadata
	General struct {
		PublicKey string `json:"publicKey"`
	} `json:"general"`
}

func (info *CreateSSHInfo) CreateSSHKey(ctx context.Context) error {
	log := logger.GetLogger(ctx)
	_, err := config.K8sClient.CoreV1().Secrets(info.Metadata.GetProjectID()).Create(ctx, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: info.Metadata.GetResourceEffectiveID(), Labels: info.Metadata.GetLabels()},
		StringData: map[string]string{
			sshDataKey: info.General.PublicKey,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		log.Err(err).Str("method", "CreateSSHKey").Str("namespace", info.Metadata.GetProjectID()).Msg("Error creating secret for ssh key")
		return err
	}

	return nil
}
