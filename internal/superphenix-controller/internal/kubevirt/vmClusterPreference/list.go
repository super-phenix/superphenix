package vmClusterPreference

import (
	"context"

	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ListVMClusterPreference(ctx context.Context) ([]string, error) {
	log := logger.GetLogger(ctx)

	list, err := k8s.VirtClient.VirtualMachineClusterPreference().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Err(err).Msg("Error getting VM Cluster Preference")
		return make([]string, 0), err
	}

	results := make([]string, 0)
	for _, item := range list.Items {
		results = append(results, item.Name)
	}
	return results, nil
}
