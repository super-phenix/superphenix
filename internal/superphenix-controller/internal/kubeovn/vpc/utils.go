package vpc

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func normalizeCIDR(cidr string) (string, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return "", err
	}
	return prefix.String(), nil
}

func fetchNatGwStaticRoutes(ctx context.Context, namespace, vpcEid string) (map[string]view.StaticRoute, error) {
	log := logger.GetLogger(ctx)
	// Fetch VpcNatGateway based on label `ovn.kubernetes.io/vpc=vpc-effective-id` where vpc-effective-id is the vpcEid given in parameters
	// and on label : `superphenix.net/projectID=namespace` where namespace is namespace parameter of the function
	labelSelector := fmt.Sprintf("ovn.kubernetes.io/vpc=%s,superphenix.net/projectID=%s", vpcEid, namespace)
	natGws, err := config.KubeOvnClient.KubeovnV1().VpcNatGateways().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Error().Str("namespace", namespace).Str("vpcEid", vpcEid).Err(err).Msg("Failed to fetch NatGW Static Routes")
		return nil, fmt.Errorf("failed to list VpcNatGateways: %w", err)
	}
	result := make(map[string]view.StaticRoute)
	// For each VpcNatGateway, build matching StaticRoute
	for _, natGw := range natGws.Items {
		lanIP := natGw.Spec.LanIP
		subnetEID := natGw.Spec.Subnet
		// V4 static route
		v4Route := view.StaticRoute{
			Policy:     "policyDst",
			CIDR:       "0.0.0.0/0",
			NextHopIP:  lanIP,
			RouteTable: subnetEID,
		}

		normCIDRv4, errCIDRv4 := normalizeCIDR(v4Route.CIDR)
		if errCIDRv4 != nil {
			log.Error().Str("namespace", namespace).Str("vpcEid", vpcEid).Err(errCIDRv4).Msg("Failed to normalize CIDR v4")
			return nil, fmt.Errorf("failed to normalize CIDR: %w", errCIDRv4)
		}
		// Add route to result map with key based on policy + cidr + routeTable value
		v4Key := fmt.Sprintf("%s-%s-%s", v4Route.Policy, normCIDRv4, v4Route.RouteTable)
		result[v4Key] = v4Route
		// V6 static route
		v6Route := view.StaticRoute{
			Policy:     "policyDst",
			CIDR:       "::/0",
			NextHopIP:  lanIP,
			RouteTable: subnetEID,
		}

		normCIDRv6, errCIDRv6 := normalizeCIDR(v6Route.CIDR)
		if errCIDRv6 != nil {
			log.Error().Str("namespace", namespace).Str("vpcEid", vpcEid).Err(errCIDRv6).Msg("Failed to normalize CIDR v6")
			return nil, fmt.Errorf("failed to normalize CIDR: %w", errCIDRv6)
		}
		// Add route to result map with key based on policy + cidr + routeTable value
		v6Key := fmt.Sprintf("%s-%s-%s", v6Route.Policy, normCIDRv6, v6Route.RouteTable)
		result[v6Key] = v6Route
	}
	return result, nil
}

func parseStaticRoutes(ctx context.Context, vpcSubnets []view.SubnetView, newSRList []view.StaticRoute, natGwSR map[string]view.StaticRoute) ([]*v1.StaticRoute, error) {
	log := logger.GetLogger(ctx)

	// Map of subnet used to retrieve subnet in the list easily
	mapSubnets := make(map[string]view.SubnetView)
	for _, subnet := range vpcSubnets {
		mapSubnets[subnet.Name] = subnet
	}

	newSrMap := map[string]view.StaticRoute{}
	for _, sr := range newSRList {
		normCIDR, err := normalizeCIDR(sr.CIDR)
		if err != nil {
			log.Error().Any("staticRoute", sr).Err(err).Msg("Failed to normalize CIDR")
			return nil, fmt.Errorf("failed to normalize CIDR: %w", err)
		}

		// Validate NextHopIP as IP
		if sr.NextHopIP != "" && net.ParseIP(sr.NextHopIP) == nil {
			err := fmt.Errorf("%s is not a valid IP address", sr.NextHopIP)
			return nil, err
		}

		// Validate Route Table value
		if mapSubnets[sr.RouteTable].Name == "" {
			return nil, fmt.Errorf("subnet %s not found for this VPC", sr.RouteTable)
		}

		// Validate NextHopIP is included in CIDR
		isIPinCIDR := false
		for _, subnetView := range mapSubnets {
			ok, err := utils.IsIPInCIDR(subnetView.Spec.CIDRBlock, sr.NextHopIP)
			if err != nil {
				log.Error().Str("nextHopIP", sr.NextHopIP).Str("CIDR", subnetView.Spec.CIDRBlock).Msg("failed check if IP is in CIDR")
			}

			if ok {
				isIPinCIDR = true
				break
			}
		}
		if !isIPinCIDR {
			return nil, fmt.Errorf("%s is not in any VPC subnets CIDR Range", sr.NextHopIP)
		}

		sr.CIDR = normCIDR
		shortKey := fmt.Sprintf("%s-%s-%s", sr.Policy, sr.CIDR, sr.RouteTable)
		fullKey := fmt.Sprintf("%s-%s-%s-%s", sr.Policy, sr.CIDR, sr.NextHopIP, sr.RouteTable)

		// If there is a matching natGw Static Route
		if v, ok := natGwSR[shortKey]; ok {
			// If same LanIP, just ignore it
			// Else we need to override the natGwSR and add the new SR to the map
			if v.NextHopIP != sr.NextHopIP {
				v.IsOverriden = true
				natGwSR[shortKey] = v
				newSrMap[fullKey] = sr
			}
		} else {
			// Just add the new SR
			newSrMap[fullKey] = sr
		}
	}

	result := make([]*v1.StaticRoute, 0)

	for _, v := range natGwSR {
		if !v.IsOverriden {
			result = append(result, &v1.StaticRoute{
				Policy:     v.Policy,
				CIDR:       v.CIDR,
				NextHopIP:  v.NextHopIP,
				RouteTable: v.RouteTable,
			})
		}
	}

	for _, v := range newSrMap {
		result = append(result, &v1.StaticRoute{
			Policy:     v.Policy,
			CIDR:       v.CIDR,
			NextHopIP:  v.NextHopIP,
			RouteTable: v.RouteTable,
		})
	}

	return result, nil
}
