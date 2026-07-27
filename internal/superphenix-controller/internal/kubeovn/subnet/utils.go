package subnet

import (
	"fmt"
	"net"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/rs/zerolog/log"
)

// parseNetworkInfo convert a protocol, ipv4 and/or ipv6 to CIDR, gateway and lanIP
//
// Note: lanIP will only be available for IPv4 (works also in Dual)
func parseNetworkInfo(protocol, ipv4, ipv6 string) (string, string, string, error) {
	var cidr string
	var gateway string
	var lanIP string
	if protocol == "Dual" {
		_, cidrV4, err := net.ParseCIDR(ipv4)
		if err != nil {
			log.Error().Err(err).Str("IPv4", ipv4).Msg("Parse IPv4 failed")
			return "", "", "", err
		}
		_, cidrV6, err := net.ParseCIDR(ipv6)
		if err != nil {
			log.Error().Err(err).Str("IPv4", ipv6).Msg("Parse IPv6 failed")
			return "", "", "", err
		}
		firstv4, lastv4 := utils.FirstAndLastAvailable(cidrV4)
		if firstv4 == nil {
			err := fmt.Errorf("failed to find gateway IPv4")
			log.Error().Err(err).Str("cidrV4", cidrV4.String()).Send()
			return "", "", "", err
		}
		firstv6, _ := utils.FirstAndLastAvailable(cidrV6)
		if firstv6 == nil {
			err := fmt.Errorf("failed to find gateway IPv6")
			log.Error().Err(err).Str("cidrV6", cidrV6.String()).Send()
			return "", "", "", err
		}

		if !isCIDRAllowed(cidrV4) {
			err := fmt.Errorf("cidr is not in allowed CIDRs")
			log.Error().Err(err).Str("cidrV4", cidrV4.String()).Send()
			return "", "", "", err
		}

		if !isCIDRAllowed(cidrV6) {
			err := fmt.Errorf("cidr is not in allowed CIDRs")
			log.Error().Err(err).Str("cidrV6", cidrV6.String()).Send()
			return "", "", "", err
		}

		cidr = fmt.Sprintf("%s,%s", cidrV4.String(), cidrV6.String())
		gateway = fmt.Sprintf("%s,%s", firstv4.String(), firstv6.String())
		lanIP = lastv4.String()
	} else if protocol == "IPv6" {
		_, cidrV6, err := net.ParseCIDR(ipv6)
		if err != nil {
			log.Error().Err(err).Str("IPv4", ipv6).Msg("Parse IPv6 failed")
			return "", "", "", err
		}
		firstv6, lastv6 := utils.FirstAndLastAvailable(cidrV6)
		if firstv6 == nil {
			err := fmt.Errorf("failed to find gateway IPv6")
			log.Error().Err(err).Str("cidrV6", cidrV6.String()).Send()
			return "", "", "", err
		}

		if !isCIDRAllowed(cidrV6) {
			err := fmt.Errorf("cidr is not in allowed CIDRs")
			log.Error().Err(err).Str("cidrV6", cidrV6.String()).Send()
			return "", "", "", err
		}

		cidr = cidrV6.String()
		gateway = firstv6.String()
		lanIP = lastv6.String()
	} else {
		_, cidrV4, err := net.ParseCIDR(ipv4)
		if err != nil {
			log.Error().Err(err).Str("IPv4", ipv4).Msg("Parse IPv4 failed")
			return "", "", "", err
		}
		firstv4, lastv4 := utils.FirstAndLastAvailable(cidrV4)
		if firstv4 == nil {
			err := fmt.Errorf("failed to find gateway IPv4")
			log.Error().Err(err).Str("cidrV4", cidrV4.String()).Send()
			return "", "", "", err
		}

		if !isCIDRAllowed(cidrV4) {
			err := fmt.Errorf("cidr is not in allowed CIDRs")
			log.Error().Err(err).Str("cidrV4", cidrV4.String()).Send()
			return "", "", "", err
		}

		cidr = cidrV4.String()
		gateway = firstv4.String()
		lanIP = lastv4.String()
	}

	return cidr, gateway, lanIP, nil
}

// parseSubnetCIDR convert a protocol, and subnet CIDR to gateway and lanIP
//
// Note: lanIP will only be available for IPv4 (works also in Dual)
func parseSubnetCIDR(protocol, cidr string) (string, string, error) {
	var gateway string
	var lanIP string
	if protocol == "Dual" {
		parts := strings.Split(cidr, ",")
		if len(parts) < 2 {
			err := fmt.Errorf("invalid Dual stack CIDR: %s", cidr)
			log.Error().Err(err).Str("cidr", cidr).Msg("Split CIDR failed")
			return "", "", err
		}
		ipv4 := parts[0]
		_, cidrV4, err := net.ParseCIDR(ipv4)
		if err != nil {
			log.Error().Err(err).Str("IPv4", ipv4).Msg("Parse IPv4 failed")
			return "", "", err
		}
		ipv6 := parts[1]
		_, cidrV6, err := net.ParseCIDR(ipv6)
		if err != nil {
			log.Error().Err(err).Str("IPv4", ipv6).Msg("Parse IPv6 failed")
			return "", "", err
		}
		firstv4, lastv4 := utils.FirstAndLastAvailable(cidrV4)
		if firstv4 == nil {
			err := fmt.Errorf("failed to find gateway IPv4")
			log.Error().Err(err).Str("cidrV4", cidrV4.String()).Send()
			return "", "", err
		}
		firstv6, _ := utils.FirstAndLastAvailable(cidrV6)
		if firstv6 == nil {
			err := fmt.Errorf("failed to find gateway IPv6")
			log.Error().Err(err).Str("cidrV6", cidrV6.String()).Send()
			return "", "", err
		}
		gateway = fmt.Sprintf("%s,%s", firstv4.String(), firstv6.String())
		lanIP = lastv4.String()
	} else if protocol == "IPv6" {
		ipv6 := cidr
		_, cidrV6, err := net.ParseCIDR(ipv6)
		if err != nil {
			log.Error().Err(err).Str("IPv4", ipv6).Msg("Parse IPv6 failed")
			return "", "", err
		}
		firstv6, _ := utils.FirstAndLastAvailable(cidrV6)
		if firstv6 == nil {
			err := fmt.Errorf("failed to find gateway IPv6")
			log.Error().Err(err).Str("cidrV6", cidrV6.String()).Send()
			return "", "", err
		}
		gateway = firstv6.String()
	} else {
		ipv4 := cidr
		_, cidrV4, err := net.ParseCIDR(ipv4)
		if err != nil {
			log.Error().Err(err).Str("IPv4", ipv4).Msg("Parse IPv4 failed")
			return "", "", err
		}
		firstv4, lastv4 := utils.FirstAndLastAvailable(cidrV4)
		if firstv4 == nil {
			err := fmt.Errorf("failed to find gateway IPv4")
			log.Error().Err(err).Str("cidrV4", cidrV4.String()).Send()
			return "", "", err
		}
		gateway = firstv4.String()
		lanIP = lastv4.String()
	}

	return gateway, lanIP, nil
}

func isCIDRAllowed(network *net.IPNet) bool {
	// Check if CIDR is allowed
	for _, allowedCIDR := range allowedCIDRs {
		if utils.ContainsNet(allowedCIDR, network) {
			return true
		}
	}

	return false
}
func isAllowedSharedSubnet(annotations map[string]string, projectID string) bool {
	allowedProjects, exists := annotations[spxId.SpxAnnotationAllowedProjects]
	if !exists {
		return false
	}
	for _, id := range strings.Split(allowedProjects, ",") {
		if strings.TrimSpace(id) == projectID {
			return true
		}
	}
	return false
}

func checkDnsIP(dnsV4, dnsV6 string) error {
	if dnsV4 != "" && net.ParseIP(dnsV4) == nil {
		err := fmt.Errorf("%s is not a valid IP address", dnsV4)
		return err
	}

	if dnsV6 != "" && net.ParseIP(dnsV6) == nil {
		err := fmt.Errorf("%s is not a valid IP address", dnsV6)
		return err
	}
	return nil
}
