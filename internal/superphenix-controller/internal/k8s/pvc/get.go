package pvc

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetPVC(ctx context.Context, namespace, name string) (view.PVCView, error) {
	log := logger.GetLogger(ctx)
	pvc, err := config.K8sClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error getting pvc")
		return view.PVCView{}, err
	}

	return view.PVCToView(*pvc), nil
}
