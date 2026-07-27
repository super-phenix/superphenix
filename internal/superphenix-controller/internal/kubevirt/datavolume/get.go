package datavolume

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDisk(ctx context.Context, namespace, name string) (view.DiskView, error) {
	log := logger.GetLogger(ctx)
	disk, err := config.VirtClient.CdiClient().CdiV1beta1().DataVolumes(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error getting disk")
		return view.DiskView{}, err
	}
	if err := utils.CheckProjectLabel(disk, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("Disk access denied")
		return view.DiskView{}, err
	}

	return view.DiskToView(*disk), nil
}
