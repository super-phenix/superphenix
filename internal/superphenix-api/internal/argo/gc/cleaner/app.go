package cleaner

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/typed/application/v1alpha1"
	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

const argoFinalizerPatch = `{"metadata": {"finalizers": ["resources-finalizer.argocd.argoproj.io"]}}`

type appCleaner struct {
	apps         v1alpha1.ArgoprojV1alpha1Interface
	opts         Options
	logger       zerolog.Logger
	resourceType string
}

// NewApp returns a Cleaner over Argo CD Applications.
func NewApp(apps v1alpha1.ArgoprojV1alpha1Interface, opts Options, logger zerolog.Logger) Cleaner {
	return &appCleaner{apps: apps, opts: opts, logger: logger, resourceType: "Argo Application"}
}

func (c *appCleaner) Clean(ctx context.Context) error {
	c.logger.Info().Str("ResourceType", c.resourceType).Msgf("Start cleaning %s", c.resourceType)

	resources, err := c.apps.Applications("").List(ctx, metav1.ListOptions{
		LabelSelector: c.opts.LabelMarkKey,
	})
	if err != nil {
		c.logger.Error().Str("ResourceType", c.resourceType).Err(err).Msg("Failed to list resources")
		return err
	}

	c.logger.Info().Str("ResourceType", c.resourceType).Msgf("Found %d marked resources in cluster", len(resources.Items))

	for _, item := range resources.Items {
		expired, err := ParseTimestamp(&item, c.opts.LabelMarkKey)
		if err != nil {
			c.logger.Error().Err(err).Str("ResourceType", c.resourceType).Str("name", item.GetName()).Str("deletionTimestamp", item.GetLabels()[c.opts.LabelMarkKey]).Msg("Failed to parse timestamp")
			continue
		}
		if !expired {
			continue
		}

		if c.opts.Debug {
			c.logger.Debug().Str("ResourceType", c.resourceType).Msgf("Deleting %s", item.GetName())
			continue
		}

		// Add finalizer to ensure Argo CD cleans up underlying resources
		if _, err := c.apps.Applications(item.GetNamespace()).Patch(ctx, item.GetName(), types.MergePatchType, []byte(argoFinalizerPatch), metav1.PatchOptions{}); err != nil {
			c.logger.Error().Err(err).Str("ResourceType", c.resourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to add finalizer")
			continue
		}

		propagation := metav1.DeletePropagationBackground
		if err := c.apps.Applications(item.GetNamespace()).Delete(ctx, item.GetName(), metav1.DeleteOptions{
			PropagationPolicy: &propagation,
		}); err != nil {
			c.logger.Error().Err(err).Str("ResourceType", c.resourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to delete item")
		}
	}

	c.logger.Info().Str("ResourceType", c.resourceType).Msgf("Cleaning %s completed", c.resourceType)

	return nil
}

func (c *appCleaner) Mark(ctx context.Context, namespace string, timestamp string) error {
	c.logger.Info().Str("ResourceType", c.resourceType).Msgf("Start labeling %s", c.resourceType)

	projectLabel := fmt.Sprintf("%s=%s", spxId.SpxLabelProjectID, namespace)
	resources, err := c.apps.Applications(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: projectLabel,
	})
	if err != nil {
		c.logger.Error().Str("ResourceType", c.resourceType).Err(err).Msg("Failed to list resources in namespace")
		return err
	}

	c.logger.Info().Str("ResourceType", c.resourceType).Msgf("Found %d resources in namespace", len(resources.Items))

	for _, item := range resources.Items {
		labels := item.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[c.opts.LabelMarkKey] = timestamp
		item.SetLabels(labels)

		if _, err := c.apps.Applications(item.GetNamespace()).Update(ctx, &item, metav1.UpdateOptions{}); err != nil {
			c.logger.Error().Err(err).Str("ResourceType", c.resourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to mark item")
			return fmt.Errorf("failed to mark item")
		}
	}

	c.logger.Info().Str("ResourceType", c.resourceType).Msgf("Labeling %s completed", c.resourceType)

	return nil
}

func (c *appCleaner) ErrorMessage() string {
	return fmt.Sprintf("Something went wrong during %s cleaning.", c.resourceType)
}
