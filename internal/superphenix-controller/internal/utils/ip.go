package utils

import (
	"bytes"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"strings"

	"github.com/rs/zerolog/log"
)

// Code from : https://github.com/apparentlymart/go-cidr/blob/v1.1.0/cidr/cidr.go

// FirstAndLastAvailable first and last available IP (excluding Range and Broadcast address)
// Network need at least 4 IP inside
func FirstAndLastAvailable(network *net.IPNet) (net.IP, net.IP) {
	mask, maxMask := network.Mask.Size()
	if mask >= maxMask-1 {
		log.Error().Str("network", network.String()).Msg("Not enough IP available")
		return nil, nil
	}

	first, last := AddressRange(network)

	firstAddr, err := netip.ParseAddr(first.String())
	if err != nil {
		log.Error().Err(err).Str("first", first.String()).Msg("First address parse error")
		return nil, nil
	}
	lastAddr, err := netip.ParseAddr(last.String())
	if err != nil {
		log.Error().Err(err).Str("last", last.String()).Msg("Last address parse error")
		return nil, nil
	}

	next := firstAddr.Next()
	prev := lastAddr.Prev()
	return net.ParseIP(next.String()), net.ParseIP(prev.String())
}

// AddressRange returns the first and last addresses in the given CIDR range.
func AddressRange(network *net.IPNet) (net.IP, net.IP) {
	// the first IP is easy
	firstIP := network.IP

	// the last IP is the network address OR NOT the mask address
	prefixLen, bits := network.Mask.Size()
	if prefixLen == bits {
		// Easy!
		// But make sure that our two slices are distinct, since they
		// would be in all other cases.
		lastIP := make([]byte, len(firstIP))
		copy(lastIP, firstIP)
		return firstIP, lastIP
	}

	firstIPInt, bits := ipToInt(firstIP)
	if firstIPInt == nil {
		return nil, nil
	}
	hostLen := uint(bits) - uint(prefixLen)
	lastIPInt := big.NewInt(1)
	lastIPInt.Lsh(lastIPInt, hostLen)
	lastIPInt.Sub(lastIPInt, big.NewInt(1))
	lastIPInt.Or(lastIPInt, firstIPInt)

	return firstIP, intToIP(lastIPInt, bits)
}

func ipToInt(ip net.IP) (*big.Int, int) {
	val := &big.Int{}
	val.SetBytes([]byte(ip))
	if len(ip) == net.IPv4len {
		return val, 32
	} else if len(ip) == net.IPv6len {
		return val, 128
	} else {
		log.Error().Err(fmt.Errorf("invalid IP address length %d", len(ip))).Msg("invalid IP address length")
		return nil, 0
	}
}

func intToIP(ipInt *big.Int, bits int) net.IP {
	ipBytes := ipInt.Bytes()
	ret := make([]byte, bits/8)
	// Pack our IP bytes into the end of the return array,
	// since big.Int.Bytes() removes front zero padding.
	for i := 1; i <= len(ipBytes); i++ {
		ret[len(ret)-i] = ipBytes[len(ipBytes)-i]
	}
	return net.IP(ret)
}

// ContainsNet returns true if net2 is a subset of (or equal to) net1.
func ContainsNet(net1, net2 *net.IPNet) bool {
	// If the two networks are different IP versions, return false
	if len(net1.IP) != len(net2.IP) {
		return false
	}
	// If the first network DOES NOT contain the first IP of the second network, return false
	if !net1.Contains(net2.IP) {
		return false
	}
	// If the first network DOES contain the first IP of the second network
	// Then check the mask size to verify network 1 is larger than network 2
	return bytes.Compare(net1.Mask, net2.Mask) <= 0
}

// IsIPInCIDR returns true if the given IP address is within the given CIDR range.
// cidr can be a single CIDR (e.g. "10.10.0.0/16") or a comma-separated list of
// CIDRs (e.g. "10.10.0.0/16,fd00:10:10::/64"). The function returns true if the
// IP is contained in at least one of the CIDRs.
func IsIPInCIDR(cidr, ip string) (bool, error) {
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return false, fmt.Errorf("invalid IP address: %s", ip)
	}

	cidrs := strings.Split(cidr, ",")
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		_, ipnet, err := net.ParseCIDR(c)
		if err != nil {
			return false, err
		}
		if ipnet.Contains(ipAddr) {
			return true, nil
		}
	}
	return false, nil
}
