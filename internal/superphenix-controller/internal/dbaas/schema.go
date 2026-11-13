package dbaas

import (
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	ClusterGVR = schema.GroupVersionResource{
		Group:    "postgresql.cnpg.io",
		Version:  "v1",
		Resource: informers.PostgresCluster,
	}
)
