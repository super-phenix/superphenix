package snat

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Get retrieve the SNAT linked to an EIP
//
// Deprecated: Get exists for compatibility and should not be used.
// EIP can now have multiple SNAT, use List to retrieve them.
func Get(ctx context.Context, name string) (*view.SNATView, error) {
	log := logger.GetLogger(ctx)
	snat, err := k8s.KubeOvnClient.KubeovnV1().IptablesSnatRules().Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		log.Err(err).Str("name", name).Msg("Error getting SNAT")
		return nil, err
	}
	snatView := view.SnatToView(*snat)
	return &snatView, nil
}
