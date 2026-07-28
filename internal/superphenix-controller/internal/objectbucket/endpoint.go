package objectbucket

import (
	"context"
	"fmt"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// obEndpoint is the S3 connection info read from the ObjectBucket bound to an
// OBC. Fields are empty when the OB isn't bound or readable yet.
type obEndpoint struct {
	Endpoint   string
	Region     string
	BucketName string
}

// storeTLS is what a CephObjectStore says about its endpoint scheme.
type storeTLS struct {
	found      bool
	advertised bool  // spec.hosting.advertiseEndpoint present
	useTls     bool  // valid when advertised
	securePort int64 // valid when not advertised
}

// endpointFor returns (useTls, effectivePort, ok) for an OB recorded with
// port. A store serving a securePort gets preferred over the plain one, even
// when the OB recorded the http port. ok is false when the store wasn't
// readable and the caller must fall back to a heuristic.
func (s storeTLS) endpointFor(port int64) (bool, int64, bool) {
	if !s.found {
		return false, port, false
	}
	if s.advertised {
		return s.useTls, port, true
	}
	if s.securePort != 0 {
		return true, s.securePort, true
	}
	return false, port, true
}

// resolveOBEndpoint reads the cluster-scoped ObjectBucket referenced by the
// OBC's spec.objectBucketName and returns its S3 endpoint details. The endpoint
// is supplementary, so a missing or unreadable OB degrades to empty fields
// rather than an error (the caller falls back to the configured endpoint).
func resolveOBEndpoint(ctx context.Context, obc *unstructured.Unstructured) obEndpoint {
	log := logger.GetLogger(ctx)

	obName, _, _ := unstructured.NestedString(obc.Object, "spec", "objectBucketName")
	if obName == "" {
		return obEndpoint{}
	}

	// ObjectBucket is cluster-scoped, no namespace.
	ob, err := config.DynamicClientSet.Resource(ObjectBucketGVR).Get(ctx, obName, metav1.GetOptions{})
	if err != nil {
		log.Warn().Err(err).Str("objectBucket", obName).Msg("Failed to get ObjectBucket for endpoint")
		return obEndpoint{}
	}

	host, _, _ := unstructured.NestedString(ob.Object, "spec", "endpoint", "bucketHost")
	port, _, _ := unstructured.NestedInt64(ob.Object, "spec", "endpoint", "bucketPort")
	region, _, _ := unstructured.NestedString(ob.Object, "spec", "endpoint", "region")
	bucketName, _, _ := unstructured.NestedString(ob.Object, "spec", "endpoint", "bucketName")

	useTls, effectivePort := obScheme(ctx, ob, port)
	return obEndpoint{
		Endpoint:   composeEndpoint(host, effectivePort, useTls),
		Region:     region,
		BucketName: bucketName,
	}
}

// obScheme decides the scheme and port for an OB's endpoint by asking its
// CephObjectStore. When the store can't be resolved, falls back to the
// port-443 heuristic.
func obScheme(ctx context.Context, ob *unstructured.Unstructured, port int64) (bool, int64) {
	st := lookupStoreTLS(ctx, ob)
	if useTls, effectivePort, ok := st.endpointFor(port); ok {
		return useTls, effectivePort
	}
	return port == 443, port
}

// lookupStoreTLS finds the CephObjectStore referenced by the OB's
// additionalState and reports its TLS config.
func lookupStoreTLS(ctx context.Context, ob *unstructured.Unstructured) storeTLS {
	name, _, _ := unstructured.NestedString(ob.Object, "spec", "additionalState", "objectStoreName")
	namespace, _, _ := unstructured.NestedString(ob.Object, "spec", "additionalState", "objectStoreNamespace")
	if name == "" || namespace == "" {
		return storeTLS{}
	}

	return fetchStoreTLS(ctx, namespace, name)
}

// fetchStoreTLS reads the store from the informer cache; the endpoint scheme
// is supplementary, so a missing informer or store degrades to storeTLS{} and
// the caller's heuristic.
func fetchStoreTLS(ctx context.Context, namespace, name string) storeTLS {
	log := logger.GetLogger(ctx)

	watcher, ok := informers.WatcherSet[informers.CephObjectStore]
	if !ok {
		log.Warn().Msg("cephobjectstore informer not initialized, falling back to port heuristic")
		return storeTLS{}
	}

	item, exists, err := watcher.GetByKey(namespace + "/" + name)
	if !exists && err == nil {
		// older provisioners record the name as "<namespace>.<name>"
		if trimmed, ok := strings.CutPrefix(name, namespace+"."); ok {
			item, exists, err = watcher.GetByKey(namespace + "/" + trimmed)
		}
	}
	if err != nil || !exists {
		log.Warn().Err(err).Str("namespace", namespace).Str("objectStore", name).Msg("Failed to get CephObjectStore for endpoint scheme")
		return storeTLS{}
	}

	store, ok := item.(*unstructured.Unstructured)
	if !ok {
		return storeTLS{}
	}

	if adv, found, _ := unstructured.NestedMap(store.Object, "spec", "hosting", "advertiseEndpoint"); found && adv != nil {
		useTls, _ := adv["useTls"].(bool)
		return storeTLS{found: true, advertised: true, useTls: useTls}
	}

	securePort, _, _ := unstructured.NestedInt64(store.Object, "spec", "gateway", "securePort")
	return storeTLS{found: true, securePort: securePort}
}

// composeEndpoint builds an S3 URL from the OB host/port and the store's TLS
// flag. The :port is omitted for the scheme's default port. Empty host yields "".
func composeEndpoint(host string, port int64, useTls bool) string {
	if host == "" {
		return ""
	}
	scheme, defaultPort := "http", int64(80)
	if useTls {
		scheme, defaultPort = "https", int64(443)
	}
	if port == 0 || port == defaultPort {
		return fmt.Sprintf("%s://%s", scheme, host)
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, port)
}
