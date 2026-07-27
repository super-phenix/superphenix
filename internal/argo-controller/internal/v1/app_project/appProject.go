package appProject

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateAppProjectIfNotExists(ctx context.Context, orgaId, projectId string) error {
	log := logger.GetLogger(ctx)
	namespace := fmt.Sprintf(`%s-%s`, config.Global.SpxPrefix, projectId)
	name := fmt.Sprintf("spx-%s", projectId)

	description := fmt.Sprintf("Project to deploy Superphénix resources in project spx-%s", projectId)

	m := spxId.Metadata{
		OrgId:     orgaId,
		ProjectId: projectId,
	}

	appProject := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: config.Global.AppProjectNamespace,
			Labels:    m.GetLabels(),
			Finalizers: []string{
				"resources-finalizer.argocd.argoproj.io",
			},
		},
		Spec: v1alpha1.AppProjectSpec{
			// Only Applications deployed in this namespace can use this Project
			SourceNamespaces:         []string{name},
			SourceRepos:              []string{"*"},
			ClusterResourceWhitelist: []metav1.GroupKind{{Group: "*", Kind: "*"}},
			Description:              description,
			//  Only permit applications to deploy to Superphenix clusters in their project namespace
			Destinations: []v1alpha1.ApplicationDestination{
				{Name: "spx-*", Namespace: namespace},
				{Name: "spx-*", Namespace: "velero-system"},
			},
		},
	}

	_, err := config.ArgoClient.AppProjects(config.Global.AppProjectNamespace).Create(ctx, appProject, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info().Str("namespace", namespace).Msg("app project already exists")
			return updateAppProject(ctx, namespace, appProject)
		}

		log.Err(err).Str("method", "CreateAppProjectIfNotExists").Msg("Error creating app project")
		return err
	}

	return nil
}

func updateAppProject(ctx context.Context, namespace string, appProject *v1alpha1.AppProject) error {
	log := logger.GetLogger(ctx)
	project, err := config.ArgoClient.AppProjects(config.Global.AppProjectNamespace).Get(ctx, appProject.Name, metav1.GetOptions{})

	if err != nil {
		log.Err(err).Str("namespace", appProject.Namespace).Msg("Error getting app project")
		return err
	}

	project.Spec.SourceNamespaces = appProject.Spec.SourceNamespaces
	project.Spec.SourceRepos = appProject.Spec.SourceRepos
	project.Spec.ClusterResourceWhitelist = appProject.Spec.ClusterResourceWhitelist
	project.Spec.Description = appProject.Spec.Description
	project.Spec.Destinations = appProject.Spec.Destinations

	if _, err := config.ArgoClient.AppProjects(config.Global.AppProjectNamespace).Update(ctx, project, metav1.UpdateOptions{}); err != nil {
		log.Warn().Err(err).Str("namespace", namespace).Msg("failed to update app project")
		return err
	}
	return nil
}
