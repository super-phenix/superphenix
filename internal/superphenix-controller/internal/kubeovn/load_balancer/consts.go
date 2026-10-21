package loadBalancer

const (
	logicalRouterAnnotation = "ovn.kubernetes.io/logical_router"
	logicalSwitchAnnotation = "ovn.kubernetes.io/logical_switch"
	serviceVPCAnnotation    = "ovn.kubernetes.io/vpc"
	defaultVPC              = "ovn-cluster"
	ruleServicePrefix       = "slr-"
)
