package loadBalancer

import "github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

const (
	logicalRouterAnnotation = utils.LogicalRouterAnnotation
	logicalSwitchAnnotation = utils.LogicalSwitchAnnotation
	serviceVPCAnnotation    = "ovn.kubernetes.io/vpc"
	defaultVPC              = "ovn-cluster"
	ruleServicePrefix       = "slr-"
)
