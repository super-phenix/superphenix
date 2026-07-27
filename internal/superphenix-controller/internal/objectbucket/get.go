package objectbucket

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetBucket(ctx context.Context, namespace, effectiveId string) (view.Bucket, error) {
	obc, err := config.DynamicClientSet.Resource(ObjectBucketClaimGVR).Namespace(namespace).Get(ctx, NamePrefix+effectiveId, metav1.GetOptions{})
	if err != nil {
		return view.Bucket{}, err
	}

	if err := utils.CheckProjectLabel(obc, namespace); err != nil {
		return view.Bucket{}, err
	}

	endpoint := endpointOrFallback(resolveOBEndpoint(ctx, obc).Endpoint)
	return view.BucketToResource(obc, endpoint), nil
}

func ListBuckets(ctx context.Context, namespace string) ([]view.Bucket, error) {
	log := logger.GetLogger(ctx)

	watcher, ok := informers.WatcherSet[informers.ObjectBucketClaim]
	if !ok {
		return nil, fmt.Errorf("objectbucketclaim informer not initialized")
	}

	list, err := watcher.ByIndex("namespace", namespace)
	if err != nil {
		log.Err(err).Str("namespace", namespace).Msg("Error ListBuckets")
		return nil, err
	}

	buckets := make([]view.Bucket, 0, len(list))
	for _, item := range list {
		obc, ok := item.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		obName, _, _ := unstructured.NestedString(obc.Object, "spec", "objectBucketName")
		endpoint := endpointOrFallback(getOBEndpoint(ctx, obName))
		buckets = append(buckets, view.BucketToResource(obc, endpoint))
	}
	return buckets, nil
}

// getOBEndpoint returns the composed S3 endpoint for one ObjectBucket
// if the OB can't be resolved it returns ""
func getOBEndpoint(ctx context.Context, obName string) string {
	log := logger.GetLogger(ctx)

	if obName == "" {
		return ""
	}
	watcher, ok := informers.WatcherSet[informers.ObjectBucket]
	if !ok {
		log.Warn().Msg("objectbucket informer not initialized, falling back to configured endpoint")
		return ""
	}
	// ObjectBucket is cluster-scoped, the cache key is the bare name.
	item, exists, err := watcher.GetByKey(obName)
	if err != nil || !exists {
		return ""
	}
	ob, ok := item.(*unstructured.Unstructured)
	if !ok {
		return ""
	}
	host, _, _ := unstructured.NestedString(ob.Object, "spec", "endpoint", "bucketHost")
	port, _, _ := unstructured.NestedInt64(ob.Object, "spec", "endpoint", "bucketPort")
	useTls, effectivePort := obScheme(ctx, ob, port)
	return composeEndpoint(host, effectivePort, useTls)
}

// endpointOrFallback returns endpoint, or the configured ExternalEndpoint when
// the OB endpoint couldn't be resolved.
func endpointOrFallback(endpoint string) string {
	if endpoint != "" {
		return endpoint
	}
	return config.Global.S3.ExternalEndpoint
}
