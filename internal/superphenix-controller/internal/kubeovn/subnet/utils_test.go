package subnet

import (
	"net"
	"reflect"
	"testing"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func TestParseNetworkInfo(t *testing.T) {
	tests := []struct {
		name        string
		protocol    string
		ipv4        string
		ipv6        string
		wantCidr    string
		wantGateway string
		wantLanIP   string
		wantErr     bool
	}{
		{
			name:        "IPv4 valid",
			protocol:    "IPv4",
			ipv4:        "192.168.1.0/24",
			ipv6:        "",
			wantCidr:    "192.168.1.0/24",
			wantGateway: "192.168.1.1",
			wantLanIP:   "192.168.1.254",
			wantErr:     false,
		},
		{
			name:        "IPv6 valid",
			protocol:    "IPv6",
			ipv4:        "",
			ipv6:        "fc00::/64",
			wantCidr:    "fc00::/64",
			wantGateway: "fc00::1",
			wantLanIP:   "fc00::ffff:ffff:ffff:fffe",
			wantErr:     false,
		},
		{
			name:        "Dual valid",
			protocol:    "Dual",
			ipv4:        "192.168.1.0/24",
			ipv6:        "fc00::/64",
			wantCidr:    "192.168.1.0/24,fc00::/64",
			wantGateway: "192.168.1.1,fc00::1",
			wantLanIP:   "192.168.1.254",
			wantErr:     false,
		},
		{
			name:        "IPv4 invalid CIDR",
			protocol:    "IPv4",
			ipv4:        "192.168.1.256/24",
			ipv6:        "",
			wantCidr:    "",
			wantGateway: "",
			wantLanIP:   "",
			wantErr:     true,
		},
		{
			name:        "IPv4 not allowed",
			protocol:    "IPv4",
			ipv4:        "8.8.8.0/24",
			ipv6:        "",
			wantCidr:    "",
			wantGateway: "",
			wantLanIP:   "",
			wantErr:     true,
		},
		{
			name:        "IPv6 not allowed",
			protocol:    "IPv6",
			ipv4:        "",
			ipv6:        "2001:db8::/32",
			wantCidr:    "",
			wantGateway: "",
			wantLanIP:   "",
			wantErr:     true,
		},
		{
			name:        "Dual IPv4 not allowed",
			protocol:    "Dual",
			ipv4:        "8.8.8.0/24",
			ipv6:        "fc00::/64",
			wantCidr:    "",
			wantGateway: "",
			wantLanIP:   "",
			wantErr:     true,
		},
		{
			name:        "Dual IPv6 not allowed",
			protocol:    "Dual",
			ipv4:        "192.168.1.0/24",
			ipv6:        "2001:db8::/32",
			wantCidr:    "",
			wantGateway: "",
			wantLanIP:   "",
			wantErr:     true,
		},
		{
			name:        "IPv4 small subnet (no gateway)",
			protocol:    "IPv4",
			ipv4:        "192.168.1.0/31",
			ipv6:        "",
			wantCidr:    "",
			wantGateway: "",
			wantLanIP:   "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cidr, gateway, lanIP, err := parseNetworkInfo(tt.protocol, tt.ipv4, tt.ipv6)
			if !reflect.DeepEqual(err != nil, tt.wantErr) {
				t.Errorf("parseNetworkInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if !reflect.DeepEqual(cidr, tt.wantCidr) {
					t.Errorf("parseNetworkInfo() cidr = %v, want %v", cidr, tt.wantCidr)
				}
				if !reflect.DeepEqual(gateway, tt.wantGateway) {
					t.Errorf("parseNetworkInfo() gateway = %v, want %v", gateway, tt.wantGateway)
				}
				if !reflect.DeepEqual(lanIP, tt.wantLanIP) {
					t.Errorf("parseNetworkInfo() lanIP = %v, want %v", lanIP, tt.wantLanIP)
				}
			}
		})
	}
}

func TestParseSubnetCIDR(t *testing.T) {
	tests := []struct {
		name        string
		protocol    string
		cidr        string
		wantGateway string
		wantLanIP   string
		wantErr     bool
	}{
		{
			name:        "IPv4 valid",
			protocol:    "IPv4",
			cidr:        "192.168.1.0/24",
			wantGateway: "192.168.1.1",
			wantLanIP:   "192.168.1.254",
			wantErr:     false,
		},
		{
			name:        "IPv6 valid",
			protocol:    "IPv6",
			cidr:        "fc00::/64",
			wantGateway: "fc00::1",
			wantLanIP:   "",
			wantErr:     false,
		},
		{
			name:        "Dual valid",
			protocol:    "Dual",
			cidr:        "192.168.1.0/24,fc00::/64",
			wantGateway: "192.168.1.1,fc00::1",
			wantLanIP:   "192.168.1.254",
			wantErr:     false,
		},
		{
			name:        "IPv4 invalid CIDR",
			protocol:    "IPv4",
			cidr:        "invalid",
			wantGateway: "",
			wantLanIP:   "",
			wantErr:     true,
		},
		{
			name:        "Dual missing IPv6",
			protocol:    "Dual",
			cidr:        "192.168.1.0/24",
			wantGateway: "",
			wantLanIP:   "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway, lanIP, err := parseSubnetCIDR(tt.protocol, tt.cidr)
			if !reflect.DeepEqual(err != nil, tt.wantErr) {
				t.Errorf("parseSubnetCIDR() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if !reflect.DeepEqual(gateway, tt.wantGateway) {
					t.Errorf("parseSubnetCIDR() gateway = %v, want %v", gateway, tt.wantGateway)
				}
				if !reflect.DeepEqual(lanIP, tt.wantLanIP) {
					t.Errorf("parseSubnetCIDR() lanIP = %v, want %v", lanIP, tt.wantLanIP)
				}
			}
		})
	}
}

func TestIsAllowedSharedSubnet(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		projectID   string
		expected    bool
	}{
		{
			name:        "No annotations",
			annotations: nil,
			projectID:   "project-1",
			expected:    false,
		},
		{
			name:        "No allowedProjects annotation",
			annotations: map[string]string{"other": "value"},
			projectID:   "project-1",
			expected:    false,
		},
		{
			name:        "Single allowed project matches",
			annotations: map[string]string{spxId.SpxAnnotationAllowedProjects: "project-1"},
			projectID:   "project-1",
			expected:    true,
		},
		{
			name:        "Single allowed project does not match",
			annotations: map[string]string{spxId.SpxAnnotationAllowedProjects: "project-2"},
			projectID:   "project-1",
			expected:    false,
		},
		{
			name:        "Multiple allowed projects with match",
			annotations: map[string]string{spxId.SpxAnnotationAllowedProjects: "project-1,project-2,project-3"},
			projectID:   "project-2",
			expected:    true,
		},
		{
			name:        "Multiple allowed projects without match",
			annotations: map[string]string{spxId.SpxAnnotationAllowedProjects: "project-1,project-2,project-3"},
			projectID:   "project-4",
			expected:    false,
		},
		{
			name:        "Allowed projects with spaces around IDs",
			annotations: map[string]string{spxId.SpxAnnotationAllowedProjects: "project-1, project-2, project-3"},
			projectID:   "project-2",
			expected:    true,
		},
		{
			name:        "Empty allowedProjects annotation",
			annotations: map[string]string{spxId.SpxAnnotationAllowedProjects: ""},
			projectID:   "project-1",
			expected:    false,
		},
		{
			name:        "Partial match should not match",
			annotations: map[string]string{spxId.SpxAnnotationAllowedProjects: "project-10,project-12"},
			projectID:   "project-1",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAllowedSharedSubnet(tt.annotations, tt.projectID)
			if got != tt.expected {
				t.Errorf("isAllowedSharedSubnet() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsCIDRAllowed(t *testing.T) {
	tests := []struct {
		name    string
		cidr    string
		allowed bool
	}{
		{"10.0.0.0/8 is allowed", "10.0.0.0/8", true},
		{"10.1.0.0/16 is allowed (subset of 10.0.0.0/8)", "10.1.0.0/16", true},
		{"172.16.0.0/12 is allowed", "172.16.0.0/12", true},
		{"172.17.0.0/16 is allowed (subset)", "172.17.0.0/16", true},
		{"192.168.0.0/16 is allowed", "192.168.0.0/16", true},
		{"192.168.1.0/24 is allowed (subset)", "192.168.1.0/24", true},
		{"224.0.0.0/4 is allowed", "224.0.0.0/4", true},
		{"240.0.0.0/4 is allowed", "240.0.0.0/4", true},
		{"fc00::/7 is allowed", "fc00::/7", true},
		{"fc00:1::/64 is allowed (subset)", "fc00:1::/64", true},
		{"64:ff9b:1::/48 is allowed", "64:ff9b:1::/48", true},
		{"8.8.8.8/32 is NOT allowed", "8.8.8.8/32", false},
		{"1.1.1.1/32 is NOT allowed", "1.1.1.1/32", false},
		{"2001:db8::/32 is NOT allowed", "2001:db8::/32", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ipNet, _ := net.ParseCIDR(tt.cidr)
			if got := isCIDRAllowed(ipNet); !reflect.DeepEqual(got, tt.allowed) {
				t.Errorf("isCIDRAllowed() = %v, want %v", got, tt.allowed)
			}
		})
	}
}
