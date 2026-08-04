package argo

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/argo/view"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// DefaultSyncPolicy is applied to every Application this package creates or updates.
var DefaultSyncPolicy = v1alpha1.SyncPolicy{
	Automated: &v1alpha1.SyncPolicyAutomated{
		SelfHeal: true,
		Prune:    true,
	},
	SyncOptions: []string{
		"CreateNamespace=false",
		"ApplyOutOfSyncOnly=true",
		"RespectIgnoreDifferences=true",
	},
}

// AppSource describes where an Application's manifests come from. Helm and Plugin
// are mutually exclusive; Plugin wins when its Name is set.
type AppSource struct {
	RepoURL        string                           `json:"repoUrl"`
	TargetRevision string                           `json:"targetRevision"`
	Chart          string                           `json:"chart,omitempty"`
	Path           string                           `json:"path,omitempty"`
	Helm           v1alpha1.ApplicationSourceHelm   `json:"helm"`
	Plugin         v1alpha1.ApplicationSourcePlugin `json:"plugin"`
}

// AppGeneral holds the Application identity fields.
type AppGeneral struct {
	AppName     string `json:"appName"`
	Destination string `json:"destination"`
}

// AppSpec is the mutable part of an Application: everything an update may change.
type AppSpec struct {
	Source            AppSource                  `json:"source"`
	IgnoreDifferences v1alpha1.IgnoreDifferences `json:"ignoreDifferences,omitempty"`
}

// CreateAppInfo is the full description of an Application to create.
type CreateAppInfo struct {
	spxId.Metadata
	General AppGeneral `json:"general"`
	Spec    AppSpec    `json:"spec"`
}

// UpdateAppInfo is the spec half of CreateAppInfo — the only part an update touches.
type UpdateAppInfo = AppSpec

const argoFinalizerPatch = `{"metadata": {"finalizers": ["resources-finalizer.argocd.argoproj.io"]}}`

// CreateApp provisions everything an Application needs: the project namespace,
// the AppProject scoping it, and the Application itself. The first two are
// idempotent, so callers only ever need this one entry point.
func (c *Client) CreateApp(ctx context.Context, info CreateAppInfo) error {
	log := logger.GetLogger(ctx)

	if err := info.Metadata.ConvertToSpxMetadata(info.Metadata); err != nil {
		log.Err(err).Any("info", info).Msg("Failed to parse app info")
		return err
	}
	namespace := info.GetProjectID()

	if err := c.ensureNamespace(ctx, info.OrgId, info.ProjectId); err != nil {
		return err
	}
	if err := c.ensureAppProject(ctx, info.OrgId, info.ProjectId); err != nil {
		return err
	}

	revisionHistory := int64(3)

	app := v1alpha1.Application{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      info.General.AppName,
			Namespace: namespace,
			Labels:    info.GetLabels(),
		},
		Spec: v1alpha1.ApplicationSpec{
			Source:            applicationSource(info.Spec.Source),
			IgnoreDifferences: info.Spec.IgnoreDifferences,
			Destination: v1alpha1.ApplicationDestination{
				Name:      info.General.Destination,
				Namespace: namespace,
			},
			Project:              namespace,
			SyncPolicy:           &DefaultSyncPolicy,
			RevisionHistoryLimit: &revisionHistory,
		},
	}

	if _, err := c.apps.Applications(namespace).Create(ctx, &app, k8smetav1.CreateOptions{}); err != nil {
		log.Err(err).Any("info", info).Msg("Failed to create app")
		return err
	}

	return nil
}

// UpdateApp rewrites an Application's source, ignore-differences and sync policy.
// Gitops-managed applications are owned by their repository and are refused.
func (c *Client) UpdateApp(ctx context.Context, name, namespace string, info UpdateAppInfo) error {
	log := logger.GetLogger(ctx)

	appToUpdate, err := c.apps.Applications(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("name", name).Msg("Error getting argo app")
		return err
	}

	if appToUpdate.GetLabels()[spxId.SpxLabelGitops] == "true" {
		log.Error().Str("name", name).Msg("Cannot update gitops resources")
		return ErrGitopsManaged
	}

	appToUpdate.Spec.Source = applicationSource(info.Source)
	appToUpdate.Spec.SyncPolicy = &DefaultSyncPolicy
	appToUpdate.Spec.IgnoreDifferences = info.IgnoreDifferences

	if _, err := c.apps.Applications(namespace).Update(ctx, appToUpdate, k8smetav1.UpdateOptions{}); err != nil {
		log.Err(err).Str("name", name).Msg("Failed to update app")
		return err
	}

	return nil
}

// GetApp returns the filtered view of a single Application.
func (c *Client) GetApp(ctx context.Context, name, namespace string) (view.AppView, error) {
	log := logger.GetLogger(ctx)

	get, err := c.apps.Applications(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("name", name).Msg("Error getting argo app")
		return view.AppView{}, err
	}
	return view.AppToView(*get), nil
}

// DeleteApp adds the Argo CD resource finalizer so the underlying workloads are
// cleaned up, then deletes the Application.
func (c *Client) DeleteApp(ctx context.Context, name, namespace string) error {
	log := logger.GetLogger(ctx)

	if _, err := c.apps.Applications(namespace).Patch(ctx, name, types.MergePatchType, []byte(argoFinalizerPatch), k8smetav1.PatchOptions{}); err != nil {
		log.Err(err).Str("name", name).Msg("Error adding finalizers on argo app")
		return err
	}

	propagation := k8smetav1.DeletePropagationBackground
	if err := c.apps.Applications(namespace).Delete(ctx, name, k8smetav1.DeleteOptions{
		PropagationPolicy: &propagation,
	}); err != nil {
		log.Err(err).Str("name", name).Msg("Error deleting argo app")
		return err
	}
	return nil
}

// applicationSource converts our source description to the Argo CD type. Helm and
// Plugin are mutually exclusive; Plugin wins when named.
func applicationSource(s AppSource) *v1alpha1.ApplicationSource {
	source := &v1alpha1.ApplicationSource{
		RepoURL:        s.RepoURL,
		TargetRevision: s.TargetRevision,
		Chart:          s.Chart,
		Path:           s.Path,
	}

	if s.Plugin.Name != "" {
		plugin := s.Plugin
		source.Plugin = &plugin
	} else {
		helm := s.Helm
		source.Helm = &helm
	}

	return source
}
