package subnet

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/alerting"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	v2 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	v3 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	"github.com/rs/zerolog"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Cleaner struct {
	ResourceType string
	Logger       zerolog.Logger
}

func (c *Cleaner) Clean(ctx context.Context) error {
	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Start cleaning %s", c.ResourceType)

	watcher := informers.WatcherSet[informers.Subnet]
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
				if err := c.deleteSubnet(ctx, item); err != nil {
					c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("name", item.GetName()).Msg("Failed to delete item")
					alerting.RaiseAlert(ctx, alerting.ProcessCleaning, alerting.ErrorCleaning, c.ResourceType, item)
				}
			}
		}
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Cleaning %s completed", c.ResourceType)

	return nil
}

func (c *Cleaner) deleteSubnet(ctx context.Context, item utils.Resource) error {
	gracePeriod := int64(0)

	if err := config.VirtClient.NetworkClient().K8sCniCncfIoV1().NetworkAttachmentDefinitions(v1.NamespaceAll).Delete(ctx, item.GetName(), k8smetav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}); err != nil && !apierrors.IsNotFound(err) {
		c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("name", item.GetName()).Msg("Failed to delete network attachment definition")
		return err
	}

	if err := config.KubeOvnClient.KubeovnV1().VpcNatGateways().Delete(ctx, item.GetName(), k8smetav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}); err != nil && !apierrors.IsNotFound(err) {
		c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("name", item.GetName()).Msg("Failed to delete NatGw")
		return err
	}

	if err := config.KubeOvnClient.KubeovnV1().Subnets().Delete(ctx, item.GetName(), k8smetav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}); err != nil {
		c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("name", item.GetName()).Msg("Failed to delete Subnet")
		return err
	}
	return nil
}

func (c *Cleaner) Mark(ctx context.Context, namespace string, timestamp string) error {
	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Start labeling %s", c.ResourceType)

	// NAD
	watcherNAD := informers.WatcherSet[informers.NAD]
	nads, err := watcherNAD.ListNamespaceResource(ctx, namespace)
	if err != nil {
		c.Logger.Error().Str("ResourceType", c.ResourceType).Err(err).Msg("Failed to list NAD in namespace")
		return err
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Found %d NAD in namespace", len(nads))

	for _, item := range nads {
		labels := item.GetLabels()
		labels[config.Global.GarbageCollection.LabelMarkKey] = timestamp
		item.SetLabels(labels)
		nad, err := utils.Transform[k8smetav1.Object, v2.NetworkAttachmentDefinition](item)
		if err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to transform item")
			alerting.RaiseAlert(ctx, alerting.ProcessLabeling, alerting.ErrorLabeling, c.ResourceType, item)
			return fmt.Errorf("failed to transform item")
		}
		if _, err := config.VirtClient.NetworkClient().K8sCniCncfIoV1().NetworkAttachmentDefinitions(item.GetNamespace()).Update(ctx, &nad, k8smetav1.UpdateOptions{}); err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to mark item")
			alerting.RaiseAlert(ctx, alerting.ProcessLabeling, alerting.ErrorLabeling, c.ResourceType, item)
			return fmt.Errorf("failed to mark item")
		}
	}

	// Nat Gateway
	watcherNatGw := informers.WatcherSet[informers.NatGw]
	natGws, err := watcherNatGw.ListNamespaceResource(ctx, namespace)
	if err != nil {
		c.Logger.Error().Str("ResourceType", c.ResourceType).Err(err).Msg("Failed to list NatGW in namespace")
		return err
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Found %d NatGW in namespace", len(natGws))

	for _, item := range natGws {
		labels := item.GetLabels()
		labels[config.Global.GarbageCollection.LabelMarkKey] = timestamp
		item.SetLabels(labels)
		natGw, err := utils.Transform[k8smetav1.Object, v3.VpcNatGateway](item)
		if err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to transform item")
			alerting.RaiseAlert(ctx, alerting.ProcessLabeling, alerting.ErrorLabeling, c.ResourceType, item)
			return fmt.Errorf("failed to transform item")
		}
		if _, err := config.KubeOvnClient.KubeovnV1().VpcNatGateways().Update(ctx, &natGw, k8smetav1.UpdateOptions{}); err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", namespace).Str("name", item.GetName()).Msg("Failed to mark item")
			alerting.RaiseAlert(ctx, alerting.ProcessLabeling, alerting.ErrorLabeling, c.ResourceType, item)
			return fmt.Errorf("failed to mark item")
		}
	}
	// Subnet
	watcherSubnet := informers.WatcherSet[informers.Subnet]
	subnets, err := watcherSubnet.ListNamespaceResource(ctx, namespace)
	if err != nil {
		c.Logger.Error().Str("ResourceType", c.ResourceType).Err(err).Msg("Failed to list Subnet in namespace")
		return err
	}

	c.Logger.Info().Str("ResourceType", c.ResourceType).Msgf("Found %d Subnet in namespace", len(subnets))

	for _, item := range subnets {
		labels := item.GetLabels()
		labels[config.Global.GarbageCollection.LabelMarkKey] = timestamp
		item.SetLabels(labels)
		subnet, err := utils.Transform[k8smetav1.Object, v3.Subnet](item)
		if err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", item.GetNamespace()).Str("name", item.GetName()).Msg("Failed to transform item")
			alerting.RaiseAlert(ctx, alerting.ProcessLabeling, alerting.ErrorLabeling, c.ResourceType, item)
			return fmt.Errorf("failed to transform item")
		}
		if _, err := config.KubeOvnClient.KubeovnV1().Subnets().Update(ctx, &subnet, k8smetav1.UpdateOptions{}); err != nil {
			c.Logger.Error().Err(err).Str("ResourceType", c.ResourceType).Str("namespace", namespace).Str("name", item.GetName()).Msg("Failed to mark item")
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
