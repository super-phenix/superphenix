package objectbucket

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// UpdateBucket replaces the OBC spec.additionalConfig with the given config.
// bucketName and storageClassName are immutable and never touched.
func (info *UpdateBucketInfo) UpdateBucket(ctx context.Context, namespace, effectiveId string) error {
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

	if err := info.General.Config.validate(); err != nil {
		log.Err(err).Str("namespace", namespace).Str("eid", effectiveId).Msg("Invalid bucket config")
		return err
	}

	additionalConfig := info.General.Config.additionalConfig()
	if len(additionalConfig) == 0 {
		unstructured.RemoveNestedField(obc.Object, "spec", "additionalConfig")
	} else if err := unstructured.SetNestedMap(obc.Object, additionalConfig, "spec", "additionalConfig"); err != nil {
		return err
	}

	_, err = resources.Update(ctx, obc, metav1.UpdateOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("eid", effectiveId).Msg("Failed to update ObjectBucketClaim")
	}
	return err
}
