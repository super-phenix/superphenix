package argoApp

import (
	"context"

	"github.com/super-phenix/superphenix/internal/argo-controller/internal/v1/models/view"
	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetArgoApp(ctx context.Context, name, namespace string) (view.AppView, error) {
	log := logger.GetLogger(ctx)

	get, err := config.ArgoClient.Applications(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("name", name).Msg("Error getting argo app")
	}
	return view.AppToView(*get), err
}
