package objectbucket

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteBucket removes the OBC. The provisioner deprovisions the bucket and
// its admin user; the generated ConfigMap/Secret are cascaded by ownerReference.
func DeleteBucket(ctx context.Context, namespace, effectiveId string) error {
	log := logger.GetLogger(ctx)

	resources := config.DynamicClientSet.Resource(ObjectBucketClaimGVR).Namespace(namespace)
	obc, err := resources.Get(ctx, NamePrefix+effectiveId, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if err := utils.CheckProjectLabel(obc, namespace); err != nil {
		return err
	}
	if err := utils.IsEditAllowed(obc.GetLabels()); err != nil {
		return err
	}

	err = resources.Delete(ctx, obc.GetName(), metav1.DeleteOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("eid", effectiveId).Msg("Failed to delete ObjectBucketClaim")
	}
	return err
}
