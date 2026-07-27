package ssh

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetSSHKey(ctx context.Context, namespace, name string) (view.SSHView, error) {
	log := logger.GetLogger(ctx)
	ssh, err := k8s.K8sClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("method", "GetSSHKey").Str("namespace", namespace).Str("name", name).Msg("Error getting ssh key")
		return view.SSHView{}, err
	}
	if err := utils.CheckProjectLabel(ssh, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("SSH key access denied")
		return view.SSHView{}, err
	}
	return view.SSHToView(*ssh), nil
}
