package subnet

import (
	"fmt"
	"net"

	k8sNet "k8s.io/utils/net"
)

var (
	allowedCIDRStrs = []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"fc00::/7",
		"64:ff9b:1::/48",
	}

	allowedCIDRs []*net.IPNet
)

func init() {
	rs, err := k8sNet.ParseCIDRs(allowedCIDRStrs)
	if err != nil {
		fmt.Println("Failed to parse allowed CIDRs")
		panic(err)
	}
	allowedCIDRs = rs
}

const (
	NatGwStatusIdle   = "idle"
	NatGwStatusDelete = "delete"
	NatGwStatusCreate = "create"
	NatGwStatusUpdate = "update"

	dhcpOptionFormat = "dns_server=%s"
)
