package vpc

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CreateVPCInfo struct {
	spxId.Metadata
}

// CreateVPC instantiates on the cluster a new VPC and the Subnets contained inside
// It also provisions the NAT gateways for each Subnet if it has a NAT gateway enabled
func (v *CreateVPCInfo) CreateVPC(ctx context.Context) error {
	log := logger.GetLogger(ctx)
	namespace := v.GetProjectID()
	if err := v.Metadata.ConvertToSpxMetadata(v.Metadata); err != nil {
		log.Err(err).Str("namespace", namespace).Any("vpc", v).Msg("Failed to parse vpc info")
		return err
	}

	// The VPC we want to create
	vpc := v1.Vpc{
		ObjectMeta: metav1.ObjectMeta{Name: v.GetResourceEffectiveID(), Labels: v.GetLabels()},
		Spec: v1.VpcSpec{
			Namespaces: []string{namespace},
			//StaticRoutes: &v1.StaticRoute{
			//	Policy:     "policyDst", //
			//	CIDR:       "", // = 0.0.0.0/0 -> Everything
			//	NextHopIP:  "", // IP de la natGateway (lanIP)
			//	RouteTable: "", // effective ID du subnet
			//},
		},
	}

	// Create the VPC on the Kubernetes cluster
	if _, err := config.KubeOvnClient.KubeovnV1().Vpcs().Create(ctx, &vpc, metav1.CreateOptions{}); err != nil {
		log.Err(err).Str("namespace", namespace).Any("vpc", v).Msg("Failed to create vpc structure")
		return err
	}

	return nil
}
