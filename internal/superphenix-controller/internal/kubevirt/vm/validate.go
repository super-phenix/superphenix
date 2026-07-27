package vm

import (
	"fmt"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"
)

// validateNetworkIP checks that any provided static IP falls within the subnet CIDR.
// Returns an error prefixed "invalid network ip" so the HTTP layer can map it to 400.
func validateNetworkIP(subnetEId, cidr, ipv4, ipv6 string) error {
	for _, ip := range []string{ipv4, ipv6} {
		if ip == "" {
			continue
		}
		ok, err := utils.IsIPInCIDR(cidr, ip)
		if err != nil || !ok {
			return fmt.Errorf("invalid network ip: %s is not within subnet %s range (%s)", ip, subnetEId, cidr)
		}
	}
	return nil
}
