package subnet

import (
	"context"
	"fmt"

	natGateway "github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/nat_gateway"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubevirt/nad"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreateSubnetInfo struct {
	spxId.Metadata
	General struct {
		VpcEId string `json:"vpcEId"`
	} `json:"general"`
	Network struct {
		Private  bool   `json:"private"`
		Protocol string `json:"protocol"`
		IPv4     string `json:"ipv4,omitempty"`
		IPv6     string `json:"ipv6,omitempty"`
		DnsV4    string `json:"dnsV4,omitempty"`
		DnsV6    string `json:"dnsV6,omitempty"`
	} `json:"network"`
	NatGateway struct {
		Enable bool `json:"enable"`
	} `json:"natGateway"`
}

func (s *CreateSubnetInfo) CreateSubnet(ctx context.Context) error {
	log := logger.GetLogger(ctx)
	namespace := s.GetProjectID()
	if err := s.Metadata.ConvertToSpxMetadata(s.Metadata); err != nil {
		log.Err(err).Str("namespace", namespace).Any("vpc", s).Msg("Failed to parse vpc info")
		return err
	}

	cidr, gateway, lanIP, err := parseNetworkInfo(s.Network.Protocol, s.Network.IPv4, s.Network.IPv6)
	if err != nil {
		log.Err(err).Any("info", s).Msg("Failed to parse Subnet Network")
		return err
	}

	excludeIps := []string{}
	if s.NatGateway.Enable {
		excludeIps = append(excludeIps, lanIP)
	}

	if err := checkDnsIP(s.Network.DnsV4, s.Network.DnsV6); err != nil {
		log.Err(err).Any("info", s).Msg("Subnet Network is invalid")
		return err
	}

	// Custom DHCP
	dnsV4Server := s.Network.DnsV4
	dnsV6Server := s.Network.DnsV6

	subnet := v1.Subnet{
		ObjectMeta: metav1.ObjectMeta{Name: s.GetResourceEffectiveID(), Labels: s.GetLabels()},
		Spec: v1.SubnetSpec{
			Vpc:                  s.General.VpcEId,
			Protocol:             s.Network.Protocol, //s.Protocol,
			RouteTable:           s.GetResourceEffectiveID(),
			Namespaces:           []string{namespace},
			CIDRBlock:            cidr, //cidr,
			Gateway:              gateway,
			Default:              false,
			Provider:             fmt.Sprintf("%s.%s.ovn", s.GetResourceEffectiveID(), s.GetProjectID()),
			EnableDHCP:           true,
			DHCPv4Options:        fmt.Sprintf(dhcpOptionFormat, dnsV4Server),
			DHCPv6Options:        fmt.Sprintf(dhcpOptionFormat, dnsV6Server),
			EnableIPv6RA:         true,
			EnableMulticastSnoop: false,
			NatOutgoing:          false,
			Private:              s.Network.Private,
			ExcludeIps:           excludeIps,
		},
	}
	if !k8s.Global.ProductsConfig.Subnets.MtuAutodetection {
		subnet.Spec.Mtu = uint32(k8s.Global.ProductsConfig.Subnets.Mtu)
	}

	if _, err := k8s.KubeOvnClient.KubeovnV1().Subnets().Create(ctx, &subnet, metav1.CreateOptions{}); err != nil {
		log.Err(err).Any("info", s).Msg("Failed to create Subnet")
		return err
	}

	infoNAD := nad.CreateNADInfo{
		Namespace: namespace,
		SubnetEID: s.GetResourceEffectiveID(),
		Labels:    s.GetLabels(),
	}
	if _, err := nad.CreateNAD(ctx, infoNAD); err != nil {
		log.Err(err).Msg("Failed to create NAD")
		if err2 := k8s.KubeOvnClient.KubeovnV1().Subnets().Delete(ctx, s.GetResourceEffectiveID(), metav1.DeleteOptions{}); err2 != nil {
			log.Err(err).Msg("Failed to remove Subnet")
			return err2
		}
		return err
	}

	if s.NatGateway.Enable {
		infoGateway := natGateway.CreateNatGwInfo{
			Metadata: s.Metadata,
			LanIP:    lanIP,
			VpcEID:   s.General.VpcEId,
		}
		if err := natGateway.CreateNatGateway(ctx, infoGateway); err != nil {
			log.Err(err).Any("natGwInfo", infoGateway).Any("subnetInfo", s).Msg("Failed to create NAT Gateway")
		}
	}

	return nil
}
