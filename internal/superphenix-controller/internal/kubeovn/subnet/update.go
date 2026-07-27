package subnet

import (
	"context"
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/eip"
	natGateway "github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/nat_gateway"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	k8s "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UpdateSubnetInfo struct {
	spxId.Metadata
	Network struct {
		Private bool   `json:"private"`
		DnsV4   string `json:"dnsV4,omitempty"`
		DnsV6   string `json:"dnsV6,omitempty"`
	} `json:"network"`
	NatGateway struct {
		Enable bool `json:"enable"`
	} `json:"natGateway"`
}

func (s *UpdateSubnetInfo) UpdateSubnet(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)

	if err := checkDnsIP(s.Network.DnsV4, s.Network.DnsV6); err != nil {
		log.Err(err).Any("info", s).Msg("Subnet Network is invalid")
		return err
	}

	subnetToUpdate, err := k8s.KubeOvnClient.KubeovnV1().Subnets().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("name", name).Msg("Failed to Get Subnet to Update")
		return err
	}

	if err := utils.CheckProjectLabel(subnetToUpdate, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("Subnet access denied")
		return err
	}

	if err := utils.IsEditAllowed(subnetToUpdate.GetLabels()); err != nil {
		log.Error().Err(err).Any("info", s).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	// Used later to delete the NatGw if needed
	natGwStatus := NatGwStatusIdle

	// Update Network
	_, lanIP, err := parseSubnetCIDR(subnetToUpdate.Spec.Protocol, subnetToUpdate.Spec.CIDRBlock)
	if err != nil {
		log.Err(err).Any("info", s).Msg("Failed to parse Subnet Network")
		return err
	}
	subnetToUpdate.Spec.Private = s.Network.Private

	// Update NatGateway
	gw, err := natGateway.GetNatGw(ctx, namespace, name)
	if err != nil {
		log.Err(err).Str("name", name).Any("info", s).Msg("Error retrieving Nat Gateway")
		return err
	}

	// If we have a nat gateway and the user want to disable it
	if gw != nil && s.NatGateway.Enable == false {
		// Check if we have EIP
		hasEip, err := eip.IsEIPInSubnet(ctx, namespace, name)
		if err != nil {
			log.Err(err).Str("name", name).Any("info", s).Msg("Failed to check linked EIP")
			return err
		}

		if hasEip {
			err := fmt.Errorf("can't disable NatGateway, some EIP are still associated to it")
			log.Err(err).Msg("Failed to update Subnet")
			return err
		} else {
			subnetToUpdate.Spec.ExcludeIps = make([]string, 0)
			natGwStatus = NatGwStatusDelete
		}
	}

	if s.NatGateway.Enable {
		if gw == nil {
			natGwStatus = NatGwStatusCreate
		} else {
			natGwStatus = NatGwStatusUpdate
		}

		subnetToUpdate.Spec.ExcludeIps = []string{lanIP}
	}

	// Custom DHCP
	dnsV4Server := s.Network.DnsV4
	dnsV6Server := s.Network.DnsV6
	subnetToUpdate.Spec.DHCPv4Options = fmt.Sprintf(dhcpOptionFormat, dnsV4Server)
	subnetToUpdate.Spec.DHCPv6Options = fmt.Sprintf(dhcpOptionFormat, dnsV6Server)

	if _, err := k8s.KubeOvnClient.KubeovnV1().Subnets().Update(ctx, subnetToUpdate, metav1.UpdateOptions{}); err != nil {
		log.Err(err).Any("info", s).Msg("Failed to create Subnet")
		return err
	}

	// Update Nat Gateway
	if natGwStatus == NatGwStatusDelete {
		if err := natGateway.DeleteNatGateway(ctx, namespace, subnetToUpdate.Spec.Vpc, name); err != nil {
			log.Err(err).Any("subnetInfo", s).Msg("Failed to delete NAT Gateway")
		}
	} else if natGwStatus == NatGwStatusCreate {
		m := spxId.Metadata{}
		if err = m.ConvertToSpxMetadata(s.Metadata); err != nil {
			log.Err(err).Any("metadata", m).Any("subnetInfo", s).Msg("Failed to generate NAT Gateway metadata")
			return nil
		}
		infoGateway := natGateway.CreateNatGwInfo{
			Metadata: m,
			LanIP:    lanIP,
			VpcEID:   subnetToUpdate.Spec.Vpc,
		}
		if err := natGateway.CreateNatGateway(ctx, infoGateway); err != nil {
			log.Err(err).Any("natGwInfo", infoGateway).Any("subnetInfo", s).Msg("Failed to create NAT Gateway")
		}
	} else if natGwStatus == NatGwStatusUpdate {
		infoGateway := natGateway.UpdateNatGwInfo{
			LanIP: lanIP,
		}
		if err := infoGateway.UpdateNatGateway(ctx, namespace, name); err != nil {
			log.Err(err).Any("natGwInfo", infoGateway).Any("subnetInfo", s).Msg("Failed to update NAT Gateway")
		}
	}

	return nil
}
