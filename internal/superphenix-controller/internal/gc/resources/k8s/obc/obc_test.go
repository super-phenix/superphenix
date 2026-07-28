package obc

import (
	"context"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/testhelper"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/objectbucket"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	"testing"
	"time"

	"github.com/rs/zerolog"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func newOBC(name, namespace string, labels map[string]string) *unstructured.Unstructured {
	obc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "objectbucket.io/v1alpha1",
			"kind":       "ObjectBucketClaim",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
	obc.SetLabels(labels)
	return obc
}

func setFakeDynamicClient(t *testing.T, objects ...runtime.Object) {
	t.Helper()
	old := config.DynamicClientSet
	config.DynamicClientSet = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{objectbucket.ObjectBucketClaimGVR: "ObjectBucketClaimList"},
		objects...,
	)
	t.Cleanup(func() { config.DynamicClientSet = old })
}

func TestClean(t *testing.T) {
	labelMarkKey := "superphenix.net/markedForDeletion"
	pastTimestamp := time.Now().Add(-1 * time.Hour).Format(utils.TimestampFormat)
	futureTimestamp := time.Now().Add(1 * time.Hour).Format(utils.TimestampFormat)

	tests := []struct {
		name          string
		obcs          []*unstructured.Unstructured
		debug         bool
		wantErr       bool
		wantRemaining int
	}{
		{
			name:          "no marked OBCs",
			obcs:          []*unstructured.Unstructured{},
			wantRemaining: 0,
		},
		{
			name: "delete expired OBC",
			obcs: []*unstructured.Unstructured{
				newOBC("bucket-expired", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
			},
			wantRemaining: 0,
		},
		{
			name: "skip future OBC",
			obcs: []*unstructured.Unstructured{
				newOBC("bucket-future", "ns1", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantRemaining: 1,
		},
		{
			name: "debug mode skips deletion",
			obcs: []*unstructured.Unstructured{
				newOBC("bucket-debug", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
			},
			debug:         true,
			wantRemaining: 1,
		},
		{
			name: "mixed expired and future",
			obcs: []*unstructured.Unstructured{
				newOBC("bucket-expired", "ns1", map[string]string{labelMarkKey: pastTimestamp}),
				newOBC("bucket-future", "ns1", map[string]string{labelMarkKey: futureTimestamp}),
			},
			wantRemaining: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey
			config.Global.GarbageCollection.Debug = tt.debug

			objects := make([]runtime.Object, len(tt.obcs))
			watcherObjs := make([]interface{}, len(tt.obcs))
			for i, o := range tt.obcs {
				objects[i] = o
				watcherObjs[i] = o
			}
			setFakeDynamicClient(t, objects...)
			testhelper.SetupFakeWatcher(informers.ObjectBucketClaim, watcherObjs...)

			c := &Cleaner{
				ResourceType: "ObjectBucketClaim",
				Logger:       zerolog.Nop(),
			}

			err := c.Clean(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Clean() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			remaining, _ := config.DynamicClientSet.Resource(objectbucket.ObjectBucketClaimGVR).Namespace("ns1").List(context.Background(), k8smetav1.ListOptions{})
			if len(remaining.Items) != tt.wantRemaining {
				t.Errorf("Clean() remaining = %d, want %d", len(remaining.Items), tt.wantRemaining)
			}
		})
	}
}

func TestMark(t *testing.T) {
	labelMarkKey := "superphenix.net/markedForDeletion"
	timestamp := time.Now().Add(48 * time.Hour).Format(utils.TimestampFormat)
	projectNs := "project-1"

	tests := []struct {
		name      string
		obcs      []*unstructured.Unstructured
		wantErr   bool
		wantCount int
	}{
		{
			name: "mark matching OBCs",
			obcs: []*unstructured.Unstructured{
				newOBC("bucket-1", "ns1", map[string]string{spxId.SpxLabelProjectID: projectNs}),
			},
			wantCount: 1,
		},
		{
			name:      "no OBCs in namespace",
			obcs:      []*unstructured.Unstructured{},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.GarbageCollection.LabelMarkKey = labelMarkKey

			objects := make([]runtime.Object, len(tt.obcs))
			watcherObjs := make([]interface{}, len(tt.obcs))
			for i, o := range tt.obcs {
				objects[i] = o
				watcherObjs[i] = o
			}
			setFakeDynamicClient(t, objects...)
			testhelper.SetupFakeWatcher(informers.ObjectBucketClaim, watcherObjs...)

			c := &Cleaner{
				ResourceType: "ObjectBucketClaim",
				Logger:       zerolog.Nop(),
			}

			err := c.Mark(context.Background(), projectNs, timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("Mark() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantCount > 0 {
				obc, _ := config.DynamicClientSet.Resource(objectbucket.ObjectBucketClaimGVR).Namespace(tt.obcs[0].GetNamespace()).Get(context.Background(), tt.obcs[0].GetName(), k8smetav1.GetOptions{})
				if obc.GetLabels()[labelMarkKey] != timestamp {
					t.Errorf("Mark() label = %v, want %v", obc.GetLabels()[labelMarkKey], timestamp)
				}
			}
		})
	}
}
