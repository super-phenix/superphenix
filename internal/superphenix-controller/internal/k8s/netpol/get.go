package netpol

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetNetPol(ctx context.Context, namespace, name string) (view.NetPolView, error) {
	log := logger.GetLogger(ctx)
	fw, err := config.K8sClient.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("namespace", namespace).Str("name", name).Msg("Error getting fw")
		return view.NetPolView{}, err
	}
	if err := utils.CheckProjectLabel(fw, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("NetworkPolicy access denied")
		return view.NetPolView{}, err
	}

	return view.NetPolToView(*fw), nil
}
