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

	endpoints := listOBEndpoints(ctx)

	buckets := make([]view.Bucket, 0, len(list))
	for _, item := range list {
		obc, ok := item.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		obName, _, _ := unstructured.NestedString(obc.Object, "spec", "objectBucketName")
		endpoint := endpointOrFallback(endpoints[obName])
		buckets = append(buckets, view.BucketToResource(obc, endpoint))
	}
	return buckets, nil
}

// listOBEndpoints returns an index of ObjectBucket name -> composed S3 endpoint,
// read from the informer cache so ListBuckets makes no API calls. A missing
// informer degrades to an empty index (callers fall back to config).
func listOBEndpoints(ctx context.Context) map[string]string {
	log := logger.GetLogger(ctx)

	watcher, ok := informers.WatcherSet[informers.ObjectBucket]
	if !ok {
		log.Warn().Msg("objectbucket informer not initialized, falling back to configured endpoint")
		return nil
	}

	list := watcher.List()
	index := make(map[string]string, len(list))
	for _, item := range list {
		ob, ok := item.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		host, _, _ := unstructured.NestedString(ob.Object, "spec", "endpoint", "bucketHost")
		port, _, _ := unstructured.NestedInt64(ob.Object, "spec", "endpoint", "bucketPort")
		useTls, effectivePort := obScheme(ctx, ob, port)
		index[ob.GetName()] = composeEndpoint(host, effectivePort, useTls)
	}
	return index
}

// endpointOrFallback returns endpoint, or the configured ExternalEndpoint when
// the OB endpoint couldn't be resolved.
func endpointOrFallback(endpoint string) string {
	if endpoint != "" {
		return endpoint
	}
	return config.Global.S3.ExternalEndpoint
}
