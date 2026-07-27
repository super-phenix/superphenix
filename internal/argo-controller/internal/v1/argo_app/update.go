package argoApp

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UpdateAppInfo struct {
	Source            AppSource                  `json:"source"`
	IgnoreDifferences v1alpha1.IgnoreDifferences `json:"ignoreDifferences,omitempty"`
}

func (v *UpdateAppInfo) UpdateApp(ctx context.Context, name, namespace string) error {
	log := logger.GetLogger(ctx)

	appToUpdate, err := config.ArgoClient.Applications(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("name", name).Msg("Error getting argo app")
	}

	if appToUpdate.GetLabels()[spxId.SpxLabelGitops] == "true" {
		log.Error().Any("info", v).Msg("Cannot update gitops resources")
		return fmt.Errorf("cannot update gitops resources")
	}

	appToUpdate.Spec.Source = &v1alpha1.ApplicationSource{
		RepoURL:        v.Source.RepoURL,
		TargetRevision: v.Source.TargetRevision,
		Chart:          v.Source.Chart,
		Path:           v.Source.Path,
	}

	// Plugin and Helm are mutually exclusive
	if v.Source.Plugin.Name != "" {
		appToUpdate.Spec.Source.Plugin = &v.Source.Plugin
	} else {
		appToUpdate.Spec.Source.Helm = &v.Source.Helm
	}

	appToUpdate.Spec.SyncPolicy = &DefaultSyncPolicy

	appToUpdate.Spec.IgnoreDifferences = v.IgnoreDifferences

	if _, err := config.ArgoClient.Applications(namespace).Update(ctx, appToUpdate, k8smetav1.UpdateOptions{}); err != nil {
		log.Err(err).Any("info", v).Msg("Failed to update app")
		return err
	}

	return nil
}
