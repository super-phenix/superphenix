package argoApp

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/argo-controller/internal/v1/models/view"
	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ListApps(ctx context.Context, namespace string) []view.AppView {
	log := logger.GetLogger(ctx)
	projectLabel := fmt.Sprintf("%s=%s", spxId.SpxLabelProjectID, namespace)
	app, err := config.ArgoClient.Applications(namespace).List(ctx, v1.ListOptions{
		LabelSelector: projectLabel,
	})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error listing Argo Apps")
		return make([]view.AppView, 0)
	}
	return view.AppsToView(app.Items)
}
