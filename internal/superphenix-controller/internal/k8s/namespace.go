package k8s

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
			return nil
		} else {
			log.Err(err).Str("method", "CreateNamespaceIfNotExists").Msg("Error creating namespace")
			return err
		}
	}

	return nil
}
