package vpc

import (
	"context"
	"fmt"

	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeleteVPC(ctx context.Context, namespace string, name string) error {
	log := logger.GetLogger(ctx)
	vpc, err := GetVPC(ctx, namespace, name)
	if err != nil {
		log.Err(err).Msg("failed to retrieve Resource VPC")
		return err
	}
	if len(vpc.Status.Subnets) > 0 {
		log.Err(err).Msg("Non empty VPC, cannot delete")
		return fmt.Errorf("non empty VPC, cannot delete")
	}

	if err := k8s.KubeOvnClient.KubeovnV1().Vpcs().Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Failed to delete VPC")
		return err
	}
	return nil
}
