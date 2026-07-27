package eip

import (
	"context"
	"fmt"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/alerting"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	"github.com/rs/zerolog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Cleaner struct {
	ResourceType string
	Logger       zerolog.Logger
}

func (c *Cleaner) Clean(ctx context.Context) error {
	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Start cleaning %s", c.ResourceType)

	watcher := informers.WatcherSet[informers.EIP]
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
				if err := c.deleteEIP(ctx, item); err != nil {
					c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("name", item.GetName()).Msg("Failed to delete item")
					alerting.RaiseAlert(ctx, alerting.ProcessCleaning, alerting.ErrorCleaning, c.ResourceType, item)
				}
			}
		}
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Cleaning %s completed", c.ResourceType)

	return nil
}

func (c *Cleaner) deleteEIP(ctx context.Context, item utils.Resource) error {
	gracePeriod := int64(0)

	if err := config.KubeOvnClient.KubeovnV1().IptablesFIPRules().Delete(ctx, item.GetName(), k8smetav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}); err != nil {
		if !apierrors.IsNotFound(err) {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("name", item.GetName()).Msg("Failed to delete FIP Rule")
			return err
		}
	}

	if err := config.KubeOvnClient.KubeovnV1().IptablesSnatRules().Delete(ctx, item.GetName(), k8smetav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}); err != nil {
		if !apierrors.IsNotFound(err) {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("name", item.GetName()).Msg("Failed to delete SNAT Rule")
			return err
		}
	}

	dnatList, err := ListMarkedDnat(ctx, *c)
	if err != nil {
		c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Msg("failed to retrieve attached DNAT")
		return err
	}

	for _, dnat := range dnatList {
		if err := config.KubeOvnClient.KubeovnV1().IptablesDnatRules().Delete(ctx, dnat.GetName(), k8smetav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}); err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Any("dnatList", dnatList).Str("name", item.GetName()).Msg("Failed to delete DNAT")
			return err
		}
	}

	// Avoid error caused by Kubeovn deletion duration for FIP
	time.Sleep(1 * time.Second)

	if err := config.KubeOvnClient.KubeovnV1().IptablesEIPs().Delete(ctx, item.GetName(), k8smetav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}); err != nil {
		c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("name", item.GetName()).Msg("Failed to delete EIP")
		return err
	}

	return nil
}

func (c *Cleaner) Mark(ctx context.Context, namespace string, timestamp string) error {
	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Start labeling %s", c.ResourceType)

	// EIP
	watcherEIP := informers.WatcherSet[informers.EIP]
	eips, err := watcherEIP.ListNamespaceResource(ctx, namespace)
	if err != nil {
		c.Logger.Error().Str("ResourceType", c.ResourceType).Err(err).Msg("Failed to list EIP in namespace")
		return err
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Found %d EIP in namespace", len(eips))

	for _, item := range eips {
		labels := item.GetLabels()
		labels[config.Global.GarbageCollection.LabelMarkKey] = timestamp
		item.SetLabels(labels)
		eip, err := utils.Transform[k8smetav1.Object, v1.IptablesEIP](item)
		if err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to transform item")
			alerting.RaiseAlert(ctx, alerting.ProcessLabeling, alerting.ErrorLabeling, c.ResourceType, item)
			return fmt.Errorf("failed to transform item")
		}
		if _, err := config.KubeOvnClient.KubeovnV1().IptablesEIPs().Update(ctx, &eip, k8smetav1.UpdateOptions{}); err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to mark item")
			alerting.RaiseAlert(ctx, alerting.ProcessLabeling, alerting.ErrorLabeling, c.ResourceType, item)
			return fmt.Errorf("failed to mark item")
		}
	}

	// FIP
	watcherFIP := informers.WatcherSet[informers.FIP]
	fips, err := watcherFIP.ListNamespaceResource(ctx, namespace)
	if err != nil {
		c.Logger.Error().Str("ResourceType", c.ResourceType).Err(err).Msg("Failed to list FIP in namespace")
		return err
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Found %d FIP in namespace", len(fips))

	for _, item := range fips {
		labels := item.GetLabels()
		labels[config.Global.GarbageCollection.LabelMarkKey] = timestamp
		item.SetLabels(labels)
		fip, err := utils.Transform[k8smetav1.Object, v1.IptablesFIPRule](item)
		if err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to transform item")
			alerting.RaiseAlert(ctx, alerting.ProcessLabeling, alerting.ErrorLabeling, c.ResourceType, item)
			return fmt.Errorf("failed to transform item")
		}
		if _, err := config.KubeOvnClient.KubeovnV1().IptablesFIPRules().Update(ctx, &fip, k8smetav1.UpdateOptions{}); err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to mark item")
			alerting.RaiseAlert(ctx, alerting.ProcessLabeling, alerting.ErrorLabeling, c.ResourceType, item)
			return fmt.Errorf("failed to mark item")
		}
	}
	// SNAT
	watcherSNAT := informers.WatcherSet[informers.SNAT]
	snats, err := watcherSNAT.ListNamespaceResource(ctx, namespace)
	if err != nil {
		c.Logger.Error().Str("ResourceType", c.ResourceType).Err(err).Msg("Failed to list SNAT in namespace")
		return err
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Found %d SNAT in namespace", len(snats))

	for _, item := range snats {
		labels := item.GetLabels()
		labels[config.Global.GarbageCollection.LabelMarkKey] = timestamp
		item.SetLabels(labels)
		snat, err := utils.Transform[k8smetav1.Object, v1.IptablesSnatRule](item)
		if err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to transform item")
			alerting.RaiseAlert(ctx, alerting.ProcessLabeling, alerting.ErrorLabeling, c.ResourceType, item)
			return fmt.Errorf("failed to transform item")
		}
		if _, err := config.KubeOvnClient.KubeovnV1().IptablesSnatRules().Update(ctx, &snat, k8smetav1.UpdateOptions{}); err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to mark item")
			alerting.RaiseAlert(ctx, alerting.ProcessLabeling, alerting.ErrorLabeling, c.ResourceType, item)
			return fmt.Errorf("failed to mark item")
		}
	}
	// DNAT
	watcherDNAT := informers.WatcherSet[informers.DNAT]
	dnats, err := watcherDNAT.ListNamespaceResource(ctx, namespace)
	if err != nil {
		c.Logger.Error().Str("ResourceType", c.ResourceType).Err(err).Msg("Failed to list DNAT in namespace")
		return err
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Found %d DNAT in namespace", len(dnats))

	for _, item := range dnats {
		labels := item.GetLabels()
		labels[config.Global.GarbageCollection.LabelMarkKey] = timestamp
		item.SetLabels(labels)
		dnat, err := utils.Transform[k8smetav1.Object, v1.IptablesDnatRule](item)
		if err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to transform item")
			alerting.RaiseAlert(ctx, alerting.ProcessLabeling, alerting.ErrorLabeling, c.ResourceType, item)
			return fmt.Errorf("failed to transform item")
		}
		if _, err := config.KubeOvnClient.KubeovnV1().IptablesDnatRules().Update(ctx, &dnat, k8smetav1.UpdateOptions{}); err != nil {
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
