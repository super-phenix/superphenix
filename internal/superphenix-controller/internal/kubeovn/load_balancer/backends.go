package loadBalancer

import (
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// checkEndpointsInSubnet ensures the endpoints are in the subnet.
func checkEndpointsInSubnet(subnet *v1.Subnet, endpoints []string) error {
	for _, endpoint := range endpoints {
		if ok, err := utils.IsIPInCIDR(subnet.Spec.CIDRBlock, endpoint); err != nil || !ok {
			return apierrors.NewBadRequest(fmt.Sprintf("endpoint %q is not in subnet %q", endpoint, subnet.Name))
		}
	}
	return nil
}
