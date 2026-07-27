package fip

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Get(ctx context.Context, name string) (*view.FIPView, error) {
	log := logger.GetLogger(ctx)
	fip, err := k8s.KubeOvnClient.KubeovnV1().IptablesFIPRules().Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		log.Err(err).Str("name", name).Msg("Error getting FIP")
		return &view.FIPView{}, err
	}
	return view.FipToView(*fip), nil
}
