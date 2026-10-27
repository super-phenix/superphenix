package dbaas

import (
	"context"
	"encoding/json"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Restart rolls the instances of the Cluster, replicas first then the primary.
// The Cluster carries the sfs-dbaas edit-blocking label, so only ownership is checked.
func Restart(ctx context.Context, namespace, effectiveId string) error {
	log := logger.GetLogger(ctx)

	if _, err := getCluster(ctx, namespace, effectiveId); err != nil {
		return err
	}

	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{
				RestartAnnotation: time.Now().UTC().Format(time.RFC3339),
			},
		},
	})
	if err != nil {
		return err
	}

	_, err = config.DynamicClientSet.Resource(ClusterGVR).Namespace(namespace).Patch(ctx, effectiveId, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("effectiveId", effectiveId).Msg("Failed to restart database")
		return err
	}

	return nil
}
