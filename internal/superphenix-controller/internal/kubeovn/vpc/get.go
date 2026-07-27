package vpc

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func GetVPC(ctx context.Context, namespace, name string) (view.VPCView, error) {
	log := logger.GetLogger(ctx)
	vpc, err := k8s.KubeOvnClient.KubeovnV1().Vpcs().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("name", name).Msg("Error getting VPC")
		return view.VPCView{}, err
	}

	if vpc.GetLabels()[spxId.SpxLabelProjectID] != namespace {
		log.Warn().Str("name", name).Str("projectID", namespace).Msg("VPC access denied")
		return view.VPCView{}, apierrors.NewNotFound(schema.GroupResource{Resource: "vpc"}, name)
	}

	natGwStaticRoutes, err := fetchNatGwStaticRoutes(ctx, namespace, name)
	if err != nil {
		log.Err(err).Str("name", name).Msg("Error fetching NATGW Static Routes")
		return view.VPCView{}, err
	}

	return view.VPCToView(*vpc, natGwStaticRoutes), err
}
