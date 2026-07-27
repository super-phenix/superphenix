package argoApp

import (
	"context"

	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreateAppInfo struct {
	spxId.Metadata
	General struct {
		AppName     string `json:"appName"`
		Destination string `json:"destination"`
	} `json:"general"`
	Spec struct {
		Source            AppSource                  `json:"source"`
		IgnoreDifferences v1alpha1.IgnoreDifferences `json:"ignoreDifferences,omitempty"`
	} `json:"spec"`
}

func (v *CreateAppInfo) CreateApp(ctx context.Context) error {
	log := logger.GetLogger(ctx)
	namespace := v.GetProjectID()
	if err := v.Metadata.ConvertToSpxMetadata(v.Metadata); err != nil {
		log.Err(err).Str("namespace", namespace).Any("info", v).Msg("Failed to parse app info")
		return err
	}

	revisionHistory := int64(3)

	app := v1alpha1.Application{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      v.General.AppName,
			Namespace: namespace,
			Labels:    v.GetLabels(),
		},
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        v.Spec.Source.RepoURL,
				TargetRevision: v.Spec.Source.TargetRevision,
				Chart:          v.Spec.Source.Chart,
				Path:           v.Spec.Source.Path,
			},
			IgnoreDifferences: v.Spec.IgnoreDifferences,
			Destination: v1alpha1.ApplicationDestination{
				Name:      v.General.Destination,
				Namespace: namespace,
			},
			Project:              namespace,
			SyncPolicy:           &DefaultSyncPolicy,
			RevisionHistoryLimit: &revisionHistory,
		},
	}

	// Plugin and Helm are mutually exclusive
	if v.Spec.Source.Plugin.Name != "" {
		app.Spec.Source.Plugin = &v.Spec.Source.Plugin
	} else {
		app.Spec.Source.Helm = &v.Spec.Source.Helm
	}

	if _, err := config.ArgoClient.Applications(namespace).Create(ctx, &app, k8smetav1.CreateOptions{}); err != nil {
		log.Err(err).Any("info", v).Msg("Failed to create app")
		return err
	}

	return nil
}
