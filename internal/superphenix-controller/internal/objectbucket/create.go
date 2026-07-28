package objectbucket

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (info *CreateBucketInfo) CreateBucket(ctx context.Context, namespace string) error {
	log := logger.GetLogger(ctx)

	metadata := spxId.Metadata{}
	if err := metadata.ConvertToSpxMetadata(info.Metadata); err != nil {
		log.Err(err).Str("namespace", namespace).Any("info", info).Msg("Failed to parse bucket informations")
		return err
	}

	storageClassName, err := resolveStorageClass(info.General.StorageClass)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Any("info", info).Msg("Failed to create bucket")
		return err
	}

	if err := info.General.Config.validate(); err != nil {
		log.Err(err).Str("namespace", namespace).Any("info", info).Msg("Invalid bucket config")
		return err
	}

	eid := metadata.GetResourceEffectiveID()
	spec := map[string]interface{}{
		"bucketName":       eid,
		"storageClassName": storageClassName,
	}
	if additionalConfig := info.General.Config.additionalConfig(); len(additionalConfig) > 0 {
		spec["additionalConfig"] = additionalConfig
	}

	obc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": ObjectBucketClaimGVR.Group + "/" + ObjectBucketClaimGVR.Version,
			"kind":       "ObjectBucketClaim",
			"metadata": map[string]interface{}{
				"name":      NamePrefix + eid,
				"namespace": namespace,
			},
			"spec": spec,
		},
	}
	obc.SetLabels(metadata.GetLabels())

	_, err = config.DynamicClientSet.Resource(ObjectBucketClaimGVR).Namespace(namespace).Create(ctx, obc, metav1.CreateOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Failed to create ObjectBucketClaim")
	}
	return err
}
