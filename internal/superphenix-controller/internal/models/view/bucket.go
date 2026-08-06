package view

import (
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// BucketToResource maps an OBC to the view model. endpoint is the resolved S3
// URL (from the bound ObjectBucket, with the AZ config as fallback) and is
// passed in so this stays a pure function.

type Bucket struct {
	Resource `json:",inline"`
	Bucket   BucketView `json:"bucket"`
}

type BucketView struct {
	Name              string `json:"name"` // S3 bucket name (spec.bucketName)
	StorageClass      string `json:"storageClass"`
	Phase             string `json:"phase,omitempty"`
	Endpoint          string `json:"endpoint,omitempty"`
	MaxObjects        string `json:"maxObjects,omitempty"`
	MaxSize           string `json:"maxSize,omitempty"`
	Policy            string `json:"policy,omitempty"`
	Lifecycle         string `json:"lifecycle,omitempty"`
	CreationTimestamp string `json:"creationTimestamp,omitempty"`
}

func BucketToResource(obc *unstructured.Unstructured, endpoint string) Bucket {
	labels := obc.GetLabels()

	bucketName, _, _ := unstructured.NestedString(obc.Object, "spec", "bucketName")
	storageClassName, _, _ := unstructured.NestedString(obc.Object, "spec", "storageClassName")
	phase, _, _ := unstructured.NestedString(obc.Object, "status", "phase")
	additionalConfig, _, _ := unstructured.NestedStringMap(obc.Object, "spec", "additionalConfig")

	return Bucket{
		Resource: Resource{
			ID:          labels[spxId.SpxLabelResourceLocalID],
			EId:         labels[spxId.SpxLabelResourceEffectiveID],
			ProductName: labels[spxId.SpxLabelResourceName],
			Gitops:      labels[spxId.SpxLabelGitops],
		},
		Bucket: BucketView{
			Name:              bucketName,
			StorageClass:      convertS3StorageClassName(storageClassName),
			Phase:             phase,
			Endpoint:          endpoint,
			MaxObjects:        additionalConfig["bucketMaxObjects"],
			MaxSize:           additionalConfig["bucketMaxSize"],
			Policy:            additionalConfig["bucketPolicy"],
			Lifecycle:         additionalConfig["bucketLifecycle"],
			CreationTimestamp: obc.GetCreationTimestamp().Format("2006-01-02T15:04:05Z07:00"),
		},
	}
}

func convertS3StorageClassName(storageClassName string) string {
	for friendlyName, fullname := range config.Global.ProductsConfig.ObjectStorage.StorageClassMapping {
		if fullname == storageClassName {
			return friendlyName
		}
	}
	log.Warn().Str("storageClassName", storageClassName).Any("storageClassMapping", config.Global.ProductsConfig.ObjectStorage.StorageClassMapping).Msg("Storage class name not found")
	return "undefined-storage-class"
}
