package vpc

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/alerting"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	"github.com/rs/zerolog"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Cleaner struct {
	ResourceType string
	Logger       zerolog.Logger
}

func (c *Cleaner) Clean(ctx context.Context) error {
	gracePeriod := int64(0)

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Start cleaning %s", c.ResourceType)

	watcher := informers.WatcherSet[informers.VPC]
	resources, err := watcher.ListMarkedResource(ctx)

	if err != nil {
		c.Logger.Error().Str("ResourceType", c.ResourceType).Err(err).Msg("Failed to list resources in namespace")
		return err
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Found %d marked resources in cluster", len(resources))

	for _, item := range resources {
		ok, err := utils.ParseTimestamp(item)
		if err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("name", item.GetName()).Str("deletionTimestamp", item.GetLabels()[config.Global.GarbageCollection.LabelMarkKey]).Msg("Failed to parse timestamp")
			alerting.RaiseAlert(ctx, alerting.ProcessCleaning, alerting.ErrorParsing, c.ResourceType, item)
			continue
		}

		if ok {
			if config.Global.GarbageCollection.Debug {
				c.Logger.Debug().Str("ResourceType", c.ResourceType).Msgf("Deleting %s", item.GetName())
			} else {
				err := config.KubeOvnClient.KubeovnV1().Vpcs().Delete(ctx, item.GetName(), k8smetav1.DeleteOptions{
					GracePeriodSeconds: &gracePeriod,
				})
				if err != nil {
					c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("name", item.GetName()).Msg("Failed to delete item")
					alerting.RaiseAlert(ctx, alerting.ProcessCleaning, alerting.ErrorCleaning, c.ResourceType, item)
				}
			}
		}
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Cleaning %s completed", c.ResourceType)

	return nil
}

func (c *Cleaner) Mark(ctx context.Context, namespace string, timestamp string) error {
	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Start labeling %s", c.ResourceType)
	watcher := informers.WatcherSet[informers.VPC]

	resources, err := watcher.ListNamespaceResource(ctx, namespace)
	if err != nil {
		c.Logger.Error().Str("ResourceType", c.ResourceType).Err(err).Msg("Failed to list resources in namespace")
		return err
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Found %d resources in namespace", len(resources))

	for _, item := range resources {
		labels := item.GetLabels()
		labels[config.Global.GarbageCollection.LabelMarkKey] = timestamp
		item.SetLabels(labels)

		vpc, err := utils.Transform[k8smetav1.Object, v1.Vpc](item)
		if err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to transform item")
			alerting.RaiseAlert(ctx, alerting.ProcessLabeling, alerting.ErrorLabeling, c.ResourceType, item)
			return fmt.Errorf("failed to transform item")
		}
		if _, err := config.KubeOvnClient.KubeovnV1().Vpcs().Update(ctx, &vpc, k8smetav1.UpdateOptions{}); err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to mark item")
			alerting.RaiseAlert(ctx, alerting.ProcessLabeling, alerting.ErrorLabeling, c.ResourceType, item)
			return fmt.Errorf("failed to mark item")
		}
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Labeling %s completed", c.ResourceType)

	return nil
}

func (c *Cleaner) ErrorMessage() string {
	return fmt.Sprintf("Something went wrong during %s cleaning.", c.ResourceType)
}

func (c *Cleaner) RaiseListingFailureAlert(ctx context.Context, processType string) {
	alerting.RaiseAlert(ctx, processType, alerting.ErrorListing, c.ResourceType, nil)
}
