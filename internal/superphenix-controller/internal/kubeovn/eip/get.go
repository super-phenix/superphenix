package eip

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetEip(ctx context.Context, namespace, name string) (view.EIPView, error) {
	log := logger.GetLogger(ctx)
	eip, err := k8s.KubeOvnClient.KubeovnV1().IptablesEIPs().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("name", name).Msg("Error getting EIP")
		return view.EIPView{}, err
	}
	if err := utils.CheckProjectLabel(eip, namespace); err != nil {
		log.Warn().Str("name", name).Str("projectID", namespace).Msg("EIP access denied")
		return view.EIPView{}, err
	}
	return view.EipToView(*eip), nil
}
