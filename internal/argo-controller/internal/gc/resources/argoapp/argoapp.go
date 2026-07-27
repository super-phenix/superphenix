package argoapp

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/argo-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/config"

	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

type Cleaner struct {
	ResourceType string
	Logger       zerolog.Logger
}

func (c *Cleaner) Clean(ctx context.Context) error {
	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Start cleaning %s", c.ResourceType)

	resources, err := config.ArgoClient.Applications("").List(ctx, metav1.ListOptions{
		LabelSelector: config.Global.GarbageCollection.LabelMarkKey,
	})
	if err != nil {
		c.Logger.Error().Str("ResourceType", c.ResourceType).Err(err).Msg("Failed to list resources")
		return err
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Found %d marked resources in cluster", len(resources.Items))

	for _, item := range resources.Items {
		ok, err := utils.ParseTimestamp(&item)
		if err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("name", item.GetName()).Str("deletionTimestamp", item.GetLabels()[config.Global.GarbageCollection.LabelMarkKey]).Msg("Failed to parse timestamp")
			continue
		}

		if ok {
			if config.Global.GarbageCollection.Debug {
				c.Logger.Debug().Str("ResourceType", c.ResourceType).Msgf("Deleting %s", item.GetName())
			} else {
				// Add finalizer to ensure Argo CD cleans up underlying resources
				if _, err := config.ArgoClient.Applications(item.GetNamespace()).Patch(ctx, item.GetName(), types.MergePatchType, []byte(`{"metadata": {"finalizers": ["resources-finalizer.argocd.argoproj.io"]}}`), metav1.PatchOptions{}); err != nil {
					c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to add finalizer")
					continue
				}

				propagation := metav1.DeletePropagationBackground
				if err := config.ArgoClient.Applications(item.GetNamespace()).Delete(ctx, item.GetName(), metav1.DeleteOptions{
					PropagationPolicy: &propagation,
				}); err != nil {
					c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to delete item")
				}
			}
		}
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Cleaning %s completed", c.ResourceType)

	return nil
}

func (c *Cleaner) Mark(ctx context.Context, namespace string, timestamp string) error {
	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Start labeling %s", c.ResourceType)

	projectLabel := fmt.Sprintf("%s=%s", spxId.SpxLabelProjectID, namespace)
	resources, err := config.ArgoClient.Applications(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: projectLabel,
	})
	if err != nil {
		c.Logger.Error().Str("ResourceType", c.ResourceType).Err(err).Msg("Failed to list resources in namespace")
		return err
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Found %d resources in namespace", len(resources.Items))

	for _, item := range resources.Items {
		labels := item.GetLabels()
		labels[config.Global.GarbageCollection.LabelMarkKey] = timestamp
		item.SetLabels(labels)

		if _, err := config.ArgoClient.Applications(item.GetNamespace()).Update(ctx, &item, metav1.UpdateOptions{}); err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to mark item")
			return fmt.Errorf("failed to mark item")
		}
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Labeling %s completed", c.ResourceType)

	return nil
}

func (c *Cleaner) ErrorMessage() string {
	return fmt.Sprintf("Something went wrong during %s cleaning.", c.ResourceType)
}
