package loadBalancer

import (
	"fmt"
	"net"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type Port struct {
	// Name generated from other fields
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort"`
	Protocol   string `json:"protocol"`
}

// buildSpec validates the request fields and returns the rule spec with its parsed selector.
func buildSpec(vip, namespace string, selectors, endpoints []string, ports []Port) (v1.SwitchLBRuleSpec, map[string]string, error) {
	if err := checkVIP(vip); err != nil {
		return v1.SwitchLBRuleSpec{}, nil, err
	}

	spec := v1.SwitchLBRuleSpec{Vip: vip, Namespace: namespace}
	var selector map[string]string

	if len(selectors) > 0 {
		var err error
		selector, err = utils.ParseLabels(selectors, "")
		if err != nil {
			return v1.SwitchLBRuleSpec{}, nil, apierrors.NewBadRequest(fmt.Sprintf("invalid selectors: %v", err))
		}
		for key, value := range selector {
			spec.Selector = append(spec.Selector, fmt.Sprintf("%s:%s", key, value))
		}
	} else {
		for _, endpoint := range endpoints {
			if net.ParseIP(endpoint).To4() == nil {
				return v1.SwitchLBRuleSpec{}, nil, apierrors.NewBadRequest(fmt.Sprintf("endpoint %q is not a valid IPv4 address", endpoint))
			}
			spec.Endpoints = append(spec.Endpoints, endpoint)
		}
	}

	for _, port := range ports {
		protocol := strings.ToUpper(port.Protocol)
		spec.Ports = append(spec.Ports, v1.SwitchLBRulePort{
			Name:       fmt.Sprintf("%d-%d-%s", port.Port, port.TargetPort, strings.ToLower(protocol)),
			Port:       port.Port,
			TargetPort: port.TargetPort,
			Protocol:   protocol,
		})
	}

	return spec, selector, nil
}

func checkVIP(vip string) error {
	ok, err := utils.IsIPInCIDR(utils.LoadBalancerVIPCIDR, vip)
	if err != nil {
		return apierrors.NewBadRequest("provided vip is not a valid IP address")
	}
	if !ok {
		return apierrors.NewBadRequest("provided vip is not in allowed CIDR")
	}
	return nil
}
