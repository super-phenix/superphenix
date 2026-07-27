package ssh

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteSSHKey(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)

	sshKey, err := k8s.K8sClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("method", "DeleteSSHKey").Str("namespace", namespace).Str("name", name).Msg("Error getting ssh key")
		return err
	}

	if err := utils.CheckProjectLabel(sshKey, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("SSH key access denied")
		return err
	}

	if err := utils.IsEditAllowed(sshKey.GetLabels()); err != nil {
		log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	err = k8s.K8sClient.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		log.Err(err).Str("method", "DeleteSSHKey").Str("namespace", namespace).Str("name", name).Msg("Error deleting ssh key")
		return err
	}
	return nil
}
