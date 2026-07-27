package natGateway

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetNatGw(ctx context.Context, namespace, name string) (*view.NatGwView, error) {
	log := logger.GetLogger(ctx)
	natGw, err := k8s.KubeOvnClient.KubeovnV1().VpcNatGateways().Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		log.Err(err).Str("name", name).Msg("Error getting Nat Gateway")
		return nil, err
	}
	if err := utils.CheckProjectLabel(natGw, namespace); err != nil {
		log.Warn().Str("name", name).Str("projectID", namespace).Msg("Nat Gateway access denied")
		return nil, nil
	}
	return view.NatGwToView(*natGw), nil
}
