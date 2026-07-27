package eip

import (
	"context"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/eip/dnat"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/eip/snat"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteEip(ctx context.Context, namespace string, name string) error {
	log := logger.GetLogger(ctx)

	eipToDelete, err := k8s.KubeOvnClient.KubeovnV1().IptablesEIPs().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to get EIP")
		return err
	}

	if err := utils.CheckProjectLabel(eipToDelete, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("EIP access denied")
		return err
	}

	if err := utils.IsEditAllowed(eipToDelete.GetLabels()); err != nil {
		log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	if err := k8s.KubeOvnClient.KubeovnV1().IptablesFIPRules().Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to delete FIP Rule")
			return err
		}
	}

	snatList, err := snat.ListResources(ctx, namespace, name)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to list SNAT Rules")
		return err
	}

	for _, item := range snatList {
		if err := k8s.KubeOvnClient.KubeovnV1().IptablesSnatRules().Delete(ctx, item.Name, metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Err(err).Str("namespace", namespace).Str("name", name).Str("snat", item.Name).Msg("Failed to delete SNAT Rule")
				return err
			}
		}
	}

	dnatList, err := dnat.ListResources(ctx, namespace, name)
	if err != nil {
		log.Err(err).Msg("failed to retrieve attached DNAT")
		return err
	}

	for _, item := range dnatList {
		if err := k8s.KubeOvnClient.KubeovnV1().IptablesDnatRules().Delete(ctx, item.Name, metav1.DeleteOptions{}); err != nil {
			log.Err(err).Any("dnatList", dnatList).Str("namespace", namespace).Str("name", name).Msg("Failed to delete DNAT")
			return err
		}
	}

	// Fix delay during FIP/SNAT deletion
	time.Sleep(400 * time.Millisecond)

	if err := k8s.KubeOvnClient.KubeovnV1().IptablesEIPs().Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to delete EIP")
		return err
	}

	return nil
}
