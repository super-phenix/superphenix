package loadBalancer

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreateLoadBalancerInfo struct {
	spxId.Metadata
	VIP string `json:"vip"`
	// Selectors and Endpoints are mutually exclusives
	Selectors []string `json:"selectors"`
	Endpoints []string `json:"endpoints"` // ip list
	Ports     []struct {
		// Name generated from other fields
		Port       int32  `json:"port"`
		TargetPort int32  `json:"targetPort"`
		Protocol   string `json:"protocol"`
	} `json:"ports"`
}

const (
	allowedCIDR = "198.18.0.0/16"
)

func (info *CreateLoadBalancerInfo) CreateLoadBalancer(ctx context.Context) error {
	log := logger.GetLogger(ctx)
	// Check VIP
	vip := net.ParseIP(info.VIP)
	if vip == nil {
		err := fmt.Errorf("provided vip is not a valid IP address")
		log.Err(err).Any("info", info).Msg("Error parsing creation values - VIP invalid")
		return err
	}
	_, cidr, err := net.ParseCIDR(allowedCIDR)
	if err != nil {
		log.Err(err).Any("info", info).Msg("Error parsing allowed CIDR")
		return err
	}

	if !cidr.Contains(vip) {
		err := fmt.Errorf("provided vip is not in allowed CIDR")
		log.Err(err).Any("info", info).Msg("Error parsing creation values - VIP invalid")
		return err
	}

	var endpoints []string
	var selectors []string

	if len(info.Selectors) > 0 {
		labelsSelector, err := utils.ParseLabels(info.Selectors, "")
		if err != nil {
			log.Err(err).Msg("Error parsing selectors")
		}
		//	Build selector
		for key, value := range labelsSelector {
			selectors = append(selectors, fmt.Sprintf("%s:%s", key, value))
		}
	} else {
		// Check each endpoint
		for _, endpoint := range info.Endpoints {
			if net.ParseIP(endpoint).To4() != nil {
				endpoints = append(endpoints, endpoint)
			} else {
				err := fmt.Errorf("endpoint is not a valid IPv4 address")
				log.Err(err).Any("info", info).Msg("Error parsing creation values - endpoints invalid")
				return err
			}
		}
	}

	var ports []v1.SwitchLBRulePort
	for _, port := range info.Ports {
		protocol := strings.ToUpper(port.Protocol)
		ports = append(ports, v1.SwitchLBRulePort{
			Name:       fmt.Sprintf("%d-%d-%s", port.Port, port.TargetPort, strings.ToLower(protocol)),
			Port:       port.Port,
			TargetPort: port.TargetPort,
			Protocol:   protocol,
		})
	}

	_, err = config.KubeOvnClient.KubeovnV1().SwitchLBRules().Create(ctx, &v1.SwitchLBRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:   info.GetResourceEffectiveID(),
			Labels: info.GetLabels(),
		},
		Spec: v1.SwitchLBRuleSpec{
			Vip:       info.VIP,
			Namespace: info.GetProjectID(),
			Selector:  selectors,
			Endpoints: endpoints,
			Ports:     ports,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}
