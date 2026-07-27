package subnet

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

func GetSubnet(ctx context.Context, name string, projectID string) (view.SubnetView, error) {
	log := logger.GetLogger(ctx)
	subnet, err := k8s.KubeOvnClient.KubeovnV1().Subnets().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("name", name).Msg("Error getting subnet")
		return view.SubnetView{}, err
	}

	isAllowedShared := isAllowedSharedSubnet(subnet.GetAnnotations(), projectID)

	if subnet.GetLabels()[spxId.SpxLabelProjectID] != projectID && !isAllowedShared {
		log.Warn().Str("name", name).Str("projectID", projectID).Msg("Subnet access denied")
		return view.SubnetView{}, apierrors.NewNotFound(schema.GroupResource{Resource: "subnet"}, name)
	}

	return view.SubnetToView(*subnet, isAllowedShared), nil
}
