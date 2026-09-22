package vm

import (
	"strings"
	"testing"
)

func TestValidateNetworkIP(t *testing.T) {
	tests := []struct {
		name        string
		cidr        string
		ipv4        string
		ipv6        string
		expectError bool
	}{
		{
			name: "no static ip (auto-assign)",
			cidr: "10.0.0.0/24",
		},
		{
			name: "in-range IPv4",
			cidr: "10.0.0.0/24",
			ipv4: "10.0.0.5",
		},
		{
			name:        "out-of-range IPv4",
			cidr:        "10.0.0.0/24",
			ipv4:        "192.168.1.5",
			expectError: true,
		},
		{
			name:        "malformed IPv4",
			cidr:        "10.0.0.0/24",
			ipv4:        "abc",
			expectError: true,
		},
		{
			name: "dual cidr, in-range v4 and v6",
			cidr: "10.0.0.0/24,fd00::/64",
			ipv4: "10.0.0.5",
			ipv6: "fd00::5",
		},
		{
			name:        "dual cidr, out-of-range v6",
			cidr:        "10.0.0.0/24,fd00::/64",
			ipv4:        "10.0.0.5",
			ipv6:        "fe80::1",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNetworkIP("subnet-eid", tt.cidr, tt.ipv4, tt.ipv6)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.HasPrefix(err.Error(), "invalid network ip") {
					t.Fatalf("expected error prefixed %q, got %q", "invalid network ip", err.Error())
				}
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestValidateNetworkMAC(t *testing.T) {
	tests := []struct {
		name        string
		mac         string
		expectError bool
	}{
		{
			name:        "empty MAC (auto-assign)",
			mac:         "",
			expectError: false,
		},
		{
			name:        "valid colon-delimited MAC",
			mac:         "52:54:00:11:22:33",
			expectError: false,
		},
		{
			name:        "reject hyphen-delimited MAC",
			mac:         "52-54-00-11-22-33",
			expectError: true,
		},
		{
			name:        "valid lowercase MAC",
			mac:         "52:54:00:ab:cd:ef",
			expectError: false,
		},
		{
			name:        "valid uppercase MAC",
			mac:         "52:54:00:AB:CD:EF",
			expectError: false,
		},
		{
			name:        "invalid short MAC",
			mac:         "52:54:00:11:22",
			expectError: true,
		},
		{
			name:        "invalid long MAC",
			mac:         "52:54:00:11:22:33:44",
			expectError: true,
		},
		{
			name:        "invalid non-hex characters",
			mac:         "52:54:00:11:22:zz",
			expectError: true,
		},
		{
			name:        "invalid random text",
			mac:         "invalid-mac-address",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNetworkMAC(tt.mac)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.HasPrefix(err.Error(), "invalid network mac") {
					t.Fatalf("expected error prefixed %q, got %q", "invalid network mac", err.Error())
				}
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
