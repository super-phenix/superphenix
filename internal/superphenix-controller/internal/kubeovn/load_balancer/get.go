package loadBalancer

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetLoadBalancer(ctx context.Context, namespace, name string) (view.LBView, error) {
	log := logger.GetLogger(ctx)
	lb, err := k8s.KubeOvnClient.KubeovnV1().SwitchLBRules().Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return view.LBView{}, err
	}
	if err != nil {
		log.Err(err).Str("name", name).Msg("Error GetNatGW")
		return view.LBView{}, err
	}
	if err := utils.CheckProjectLabel(lb, namespace); err != nil {
		log.Warn().Str("name", name).Str("projectID", namespace).Msg("LoadBalancer access denied")
		return view.LBView{}, err
	}
	return view.LBToView(*lb), nil
}
