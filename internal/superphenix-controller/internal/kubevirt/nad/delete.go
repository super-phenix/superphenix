package nad

import (
	"context"

	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteNAD(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)
	err := k8s.VirtClient.NetworkClient().K8sCniCncfIoV1().NetworkAttachmentDefinitions(namespace).Delete(ctx, name, k8smetav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		log.Warn().Err(err).Str("namespace", namespace).Str("name", name).Msg("Network attachment definition not found")
		return nil
	}
	return err
}
