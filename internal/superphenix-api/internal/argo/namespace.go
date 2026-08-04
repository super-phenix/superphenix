package argo

import (
	"context"

	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ensureNamespace creates the project namespace if it does not already exist.
func (c *Client) ensureNamespace(ctx context.Context, orgaId, projectId string) error {
	log := logger.GetLogger(ctx)
	namespace := c.Namespace(projectId)

	m := spxId.Metadata{
		OrgId:     orgaId,
		ProjectId: projectId,
	}

	nsSpec := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   namespace,
		Labels: m.GetLabels(),
	}}

	if _, err := c.k8s.CoreV1().Namespaces().Create(ctx, nsSpec, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info().Str("namespace", namespace).Msg("namespace already exists")
			return nil
		}
		log.Err(err).Str("namespace", namespace).Msg("Error creating namespace")
		return err
	}

	return nil
}
