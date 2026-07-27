package argoApp

import (
	"context"

	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func DeleteApp(ctx context.Context, name, namespace string) error {
	log := logger.GetLogger(ctx)

	if _, err := config.ArgoClient.Applications(namespace).Patch(ctx, name, types.MergePatchType, []byte(`{"metadata": {"finalizers": ["resources-finalizer.argocd.argoproj.io"]}}`), v1.PatchOptions{}); err != nil {
		log.Err(err).Str("name", name).Msg("Error adding finalizers on argo apps")
		return err
	}

	propagation := v1.DeletePropagationBackground
	if err := config.ArgoClient.Applications(namespace).Delete(ctx, name, v1.DeleteOptions{
		PropagationPolicy: &propagation,
	}); err != nil {
		log.Err(err).Str("name", name).Msg("Error deleting Argo Apps")
		return err
	}
	return nil
}
