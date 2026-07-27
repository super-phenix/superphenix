package natGateway

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/kubeovn/vpc"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreateNatGwInfo struct {
	spxId.Metadata
	LanIP  string
	VpcEID string
}

func CreateNatGateway(ctx context.Context, info CreateNatGwInfo) error {
	log := logger.GetLogger(ctx)
	bgpSpeakerDefault := config.Global.NatGatewayDefault.BgpSpeaker

	routes := make([]v1.Route, 0)
	for _, route := range config.Global.NatGatewayDefault.DefaultRoutes {
		routes = append(routes, v1.Route{
			CIDR:      route.Cidr,
			NextHopIP: route.NextHopIP,
		})
	}

	_, err := config.KubeOvnClient.KubeovnV1().VpcNatGateways().Create(ctx, &v1.VpcNatGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:   info.GetResourceEffectiveID(),
			Labels: info.GetLabels(),
		},
		Spec: v1.VpcNatGatewaySpec{
			Vpc:             info.VpcEID,
			Subnet:          info.GetResourceEffectiveID(),
			ExternalSubnets: config.Global.NatGatewayDefault.ExternalSubnets,
			LanIP:           info.LanIP,
			Routes:          routes,
			BgpSpeaker: v1.VpcBgpSpeaker{
				Enabled:               bgpSpeakerDefault.Enabled,
				ASN:                   bgpSpeakerDefault.ASN,
				RemoteASN:             bgpSpeakerDefault.RemoteASN,
				Neighbors:             bgpSpeakerDefault.Neighbors,
				HoldTime:              metav1.Duration{Duration: bgpSpeakerDefault.HoldTime},
				RouterID:              bgpSpeakerDefault.RouterID,
				Password:              bgpSpeakerDefault.Password,
				EnableGracefulRestart: bgpSpeakerDefault.EnableGracefulRestart,
				ExtraArgs:             bgpSpeakerDefault.ExtraArgs,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		log.Err(err).Msg("Error creating Nat Gateway")
		return err
	}

	if err := vpc.AddNatGateway(ctx, info.GetProjectID(), info.VpcEID, info.LanIP, info.GetResourceEffectiveID()); err != nil {
		log.Err(err).Msg("Error adding Nat Gateway to VPC")
		return err
	}

	return nil
}
