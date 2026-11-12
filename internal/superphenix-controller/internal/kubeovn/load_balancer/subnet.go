package loadBalancer

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// getRuleSubnet returns the subnet a load balancer attaches to, owned by or shared with the project.
func getRuleSubnet(ctx context.Context, name, projectID string) (*v1.Subnet, error) {
	if name == "" {
		return nil, apierrors.NewBadRequest("subnetEId is required")
	}

	subnet, err := k8s.KubeOvnClient.KubeovnV1().Subnets().Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("subnet %q not found", name))
	}
	if err != nil {
		return nil, err
	}

	if subnet.GetLabels()[spxId.SpxLabelProjectID] != projectID &&
		!utils.IsSharedWithProject(subnet.GetAnnotations(), projectID) {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("subnet %q not found", name))
	}

	if subnet.Spec.Vpc == "" || subnet.Spec.Vpc == defaultVPC {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("subnet %q is not in a project VPC", name))
	}

	return subnet, nil
}
