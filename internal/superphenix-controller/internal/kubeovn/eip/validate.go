package eip

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/eip/dnat"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var subnetResource = schema.GroupResource{Group: "kubeovn.io", Resource: "subnet"}

// getAccessibleSubnet returns the subnet an EIP is bound to (its NAT gateway
// shares the subnet name), provided the project owns it or it has been
// explicitly shared with the project. Kube-OVN objects are cluster-scoped, so
// without this check an EIP could be attached to any tenant's NAT gateway.
func getAccessibleSubnet(ctx context.Context, name, projectID string) (*v1.Subnet, error) {
	if name == "" {
		return nil, apierrors.NewNotFound(subnetResource, name)
	}

	subnet, err := k8s.KubeOvnClient.KubeovnV1().Subnets().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if subnet.GetLabels()[spxId.SpxLabelProjectID] != projectID &&
		!utils.IsSharedWithProject(subnet.GetAnnotations(), projectID) {
		return nil, apierrors.NewNotFound(subnetResource, name)
	}

	return subnet, nil
}

// validateInternalTargets ensures every internal address targeted by the EIP
// (FIP and DNAT IPs, SNAT CIDRs) lies within the subnet the EIP is bound to.
// FIP and DNAT IPs may also be a load balancer VIP.
// subnetCIDR is the Kube-OVN CIDR block, possibly dual-stack ("v4,v6").
func validateInternalTargets(subnetCIDR, internalIP string, snatCIDRs []string, dnatRules []dnat.InfoDNAT) error {
	if internalIP != "" {
		if err := checkInternalIP(subnetCIDR, internalIP); err != nil {
			return err
		}
	}

	for _, cidr := range snatCIDRs {
		if err := checkCIDRInSubnet(subnetCIDR, cidr); err != nil {
			return err
		}
	}

	for _, rule := range dnatRules {
		if err := checkInternalIP(subnetCIDR, rule.InternalIP); err != nil {
			return err
		}
	}

	return nil
}

// checkInternalIP accepts an IP of the subnet or other allowed VIP CIDR (such as LB)
func checkInternalIP(subnetCIDR, ip string) error {
	if ok, err := utils.IsIPInCIDR(utils.LoadBalancerVIPCIDR, strings.TrimSpace(ip)); err == nil && ok {
		return nil
	}
	return checkIPInSubnet(subnetCIDR, ip)
}

func checkIPInSubnet(subnetCIDR, ip string) error {
	ip = strings.TrimSpace(ip)
	ok, err := utils.IsIPInCIDR(subnetCIDR, ip)
	if err != nil || !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("internal IP %q is not in the subnet", ip))
	}
	return nil
}

func checkCIDRInSubnet(subnetCIDR, cidr string) error {
	cidr = strings.TrimSpace(cidr)
	if !strings.Contains(cidr, "/") {
		return checkIPInSubnet(subnetCIDR, cidr)
	}

	_, target, err := net.ParseCIDR(cidr)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("SNAT CIDR %q is invalid", cidr))
	}

	for _, block := range strings.Split(subnetCIDR, ",") {
		_, subnetNet, err := net.ParseCIDR(strings.TrimSpace(block))
		if err != nil {
			continue
		}
		if utils.ContainsNet(subnetNet, target) {
			return nil
		}
	}

	return apierrors.NewBadRequest(fmt.Sprintf("SNAT CIDR %q is not in the subnet", cidr))
}
