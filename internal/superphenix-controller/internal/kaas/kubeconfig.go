package kaas

import (
	"context"
	"fmt"

	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetKubeConfig(ctx context.Context, namespace, effectiveId string) ([]byte, error) {
	log := logger.GetLogger(ctx)

	secretName := fmt.Sprintf("%s-kubeconfig", effectiveId)
	kubeconfig, err := k8s.K8sClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("method", "GetKubeConfig").Str("namespace", namespace).Str("effectiveId", effectiveId).Msg("Error getting kubeconfig")
		return make([]byte, 0), err
	}

	value := kubeconfig.Data["value"]

	if value == nil {
		log.Err(err).Str("method", "GetKubeConfig").Str("namespace", namespace).Str("effectiveId", effectiveId).Msg("No value found in secret")
		return make([]byte, 0), nil
	}

	return value, nil
}
