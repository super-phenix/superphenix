package vpc

import (
	"context"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/informers"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	DefaultIPv4LanIP = "0.0.0.0/0"
	DefaultIPv6LanIP = "::/0"
)

type UpdateVPCInfo struct {
	StaticRoutes []view.StaticRoute `json:"staticRoutes"`
}

func (info *UpdateVPCInfo) UpdateVPC(ctx context.Context, namespace, name string) error {
	log := logger.GetLogger(ctx)
	vpc, err := config.KubeOvnClient.KubeovnV1().Vpcs().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("name", name).Msg("Failed to get VPC")
		return err
	}

	if err := utils.CheckProjectLabel(vpc, namespace); err != nil {
		log.Warn().Str("namespace", namespace).Str("name", name).Msg("VPC access denied")
		return err
	}

	if err := utils.IsEditAllowed(vpc.GetLabels()); err != nil {
		log.Error().Err(err).Any("info", info).Str("name", name).Msg("Edit not allowed on this resource")
		return err
	}

	return updateVPC(ctx, vpc, namespace, name, info.StaticRoutes)
}

// updateVPC take a list of new staticRoutes and compare them with all default staticRoutes implemented by VpcNatGateway
func updateVPC(ctx context.Context, vpc *v1.Vpc, namespace, name string, staticRoutes []view.StaticRoute) error {
	log := logger.GetLogger(ctx)

	natGwSR, err := fetchNatGwStaticRoutes(ctx, namespace, name)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch NatGW Static Routes")
		return err
	}

	// Fetch all subnet associated to this vpc
	subnets := listBatchSubnet(ctx, namespace, name)
	srList, err := parseStaticRoutes(ctx, subnets, staticRoutes, natGwSR)
	if err != nil {
		log.Error().Err(err).Msg("Failed parse Static Routes")
		return err
	}

	// Update Static Routes
	vpc.Spec.StaticRoutes = srList

	// Update the VPC on the Kubernetes cluster
	if _, err := config.KubeOvnClient.KubeovnV1().Vpcs().Update(ctx, vpc, metav1.UpdateOptions{}); err != nil {
		log.Err(err).Str("name", name).Any("vpc", vpc).Msg("Failed to update vpc")
		return err
	}

	return nil
}

// AddNatGateway update static routes to add the new Nat Gateway
//
// NB: We do not block updates to GitOps VPC in order to allow the Console to add a natGateway on a subnet link to the vpc
// This will cause the VPC to become unsynchronized with ArgoCD.
func AddNatGateway(ctx context.Context, namespace, vpcEID, lanIP, subnetEID string) error {
	log := logger.GetLogger(ctx)
	vpc, err := config.KubeOvnClient.KubeovnV1().Vpcs().Get(ctx, vpcEID, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("name", vpcEID).Msg("Failed to get VPC")
	}

	staticRoutes := vpc.Spec.StaticRoutes

	// Add the new static route
	staticRoutes = append(staticRoutes, &v1.StaticRoute{
		Policy:     v1.PolicyDst,     //
		CIDR:       DefaultIPv4LanIP, // = 0.0.0.0/0 -> Everything
		NextHopIP:  lanIP,            // IP de la natGateway (lanIP)
		RouteTable: subnetEID,        // effective ID du subnet
	})

	staticRoutes = append(staticRoutes, &v1.StaticRoute{
		Policy:     v1.PolicyDst,     //
		CIDR:       DefaultIPv6LanIP, // = ::/0 -> Everything
		NextHopIP:  lanIP,            // IP de la natGateway (lanIP)
		RouteTable: subnetEID,        // effective ID du subnet
	})

	newSR := make([]view.StaticRoute, 0)
	for _, sr := range staticRoutes {
		newSR = append(newSR, view.StaticRoute{
			Policy:     sr.Policy,
			CIDR:       sr.CIDR,
			NextHopIP:  sr.NextHopIP,
			RouteTable: sr.RouteTable,
		})
	}

	return updateVPC(ctx, vpc, namespace, vpcEID, newSR)
}

// RemoveNatGateway update static routes to remove a specific Nat Gateway
func RemoveNatGateway(ctx context.Context, vpcEID, subnetEID, lanIP string) error {
	log := logger.GetLogger(ctx)
	vpc, err := config.KubeOvnClient.KubeovnV1().Vpcs().Get(ctx, vpcEID, metav1.GetOptions{})
	if err != nil {
		log.Err(err).Str("name", vpcEID).Msg("Failed to get VPC")
	}

	var newStaticRoutes []*v1.StaticRoute
	for _, rs := range vpc.Spec.StaticRoutes {
		if rs.Policy == v1.PolicyDst &&
			(rs.CIDR == DefaultIPv6LanIP || rs.CIDR == DefaultIPv4LanIP) &&
			rs.NextHopIP == lanIP &&
			rs.RouteTable == subnetEID {
			// This match with default ipV4 and ipV6 static routes
			// We do not want them
			continue
		} else {
			newStaticRoutes = append(newStaticRoutes, rs)
		}
	}
	vpc.Spec.StaticRoutes = newStaticRoutes

	// Update the VPC on the Kubernetes cluster
	if _, err := config.KubeOvnClient.KubeovnV1().Vpcs().Update(ctx, vpc, metav1.UpdateOptions{}); err != nil {
		log.Err(err).Str("name", vpcEID).Any("vpc", vpc).Msg("Failed to update vpc")
		return err
	}

	return nil
}

func listBatchSubnet(ctx context.Context, namespace string, vpcEID string) []view.SubnetView {
	log := logger.GetLogger(ctx)
	list := informers.WatcherSet[informers.Subnet].List()
	subnets := make([]view.SubnetView, 0)

	for _, item := range list {
		subnet := item.(*unstructured.Unstructured)
		specVpc, found, err := unstructured.NestedString(subnet.Object, "spec", "vpc")
		if err != nil {
			log.Warn().Err(err).Str("subnet", subnet.GetName()).Msg("error reading spec.vpc from subnet")
			continue
		}
		if !found {
			log.Warn().Str("subnet", subnet.GetName()).Msg("spec.vpc not found in subnet")
			continue
		}
		if specVpc == vpcEID && subnet.GetLabels()[spxId.SpxLabelProjectID] == namespace {
			subnetView := view.UnstructuredSubnetToView(subnet, false)
			subnets = append(subnets, subnetView)
		}
	}
	return subnets
}
