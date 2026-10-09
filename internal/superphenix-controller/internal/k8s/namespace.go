package k8s

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateNamespaceIfNotExists(ctx context.Context, orgaId, projectId string) error {
	log := logger.GetLogger(ctx)
	namespace := utils.GetNamespace(projectId)

	m := spxId.Metadata{
		OrgId:     orgaId,
		ProjectId: projectId,
	}

	nsSpec := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   namespace,
		Labels: m.GetLabels(),
	}}

	_, err := config.K8sClient.CoreV1().Namespaces().Create(ctx, nsSpec, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info().Str("namespace", namespace).Msg("namespace already exists")
		} else {
			log.Err(err).Str("method", "CreateNamespaceIfNotExists").Msg("Error creating namespace")
			return err
		}
	}

	if err := EnsureResourceQuotaIfNotExists(ctx, orgaId, projectId); err != nil {
		log.Err(err).Str("method", "CreateNamespaceIfNotExists").Msg("Error ensuring resource quota")
		return err
	}

	return nil
}

func EnsureResourceQuotaIfNotExists(ctx context.Context, orgaId, projectId string) error {
	log := logger.GetLogger(ctx)
	namespace := utils.GetNamespace(projectId)
	quotaName := fmt.Sprintf("quota-%s", projectId)

	m := spxId.Metadata{
		OrgId:     orgaId,
		ProjectId: projectId,
	}

	hard := v1.ResourceList{}
	if config.Global.Quotas.DefaultAZQuotas.Pods != "" {
		if q, err := resource.ParseQuantity(config.Global.Quotas.DefaultAZQuotas.Pods); err == nil {
			hard[v1.ResourcePods] = q
		}
	}
	if config.Global.Quotas.DefaultAZQuotas.PersistentVolumeClaims != "" {
		if q, err := resource.ParseQuantity(config.Global.Quotas.DefaultAZQuotas.PersistentVolumeClaims); err == nil {
			hard[v1.ResourcePersistentVolumeClaims] = q
		}
	}
	if config.Global.Quotas.DefaultAZQuotas.VirtualMachines != "" {
		if q, err := resource.ParseQuantity(config.Global.Quotas.DefaultAZQuotas.VirtualMachines); err == nil {
			hard[v1.ResourceName("count/virtualmachines.kubevirt.io")] = q
		}
	}

	if len(hard) == 0 {
		return nil
	}

	quotaSpec := &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      quotaName,
			Namespace: namespace,
			Labels:    m.GetLabels(),
		},
		Spec: v1.ResourceQuotaSpec{
			Hard: hard,
		},
	}

	_, err := config.K8sClient.CoreV1().ResourceQuotas(namespace).Create(ctx, quotaSpec, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		log.Err(err).Str("method", "EnsureResourceQuotaIfNotExists").Msg("Error creating resource quota")
		return err
	}

	return nil
}
