package vpc

import (
	"cmp"
	"slices"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"testing"

	google_cmp "github.com/google/go-cmp/cmp"
	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
)

func TestParseStaticRoutes(t *testing.T) {
	tests := []struct {
		name       string
		vpcSubnets []view.SubnetView
		newSRList  []view.StaticRoute
		natGwSR    map[string]view.StaticRoute
		expected   []*v1.StaticRoute
		wantErr    bool
	}{
		{
			name:       "Empty inputs",
			vpcSubnets: []view.SubnetView{},
			newSRList:  []view.StaticRoute{},
			natGwSR:    map[string]view.StaticRoute{},
			expected:   []*v1.StaticRoute{},
		},
		{
			name: "Add new route, no existing NAT GW routes",
			vpcSubnets: []view.SubnetView{
				{ObjectMeta: view.ObjectMeta{Name: "rt1"}, Spec: view.SubnetSpec{CIDRBlock: "10.0.0.0/24"}},
			},
			newSRList: []view.StaticRoute{
				{Policy: "policyDst", CIDR: "1.2.3.4/32", NextHopIP: "10.0.0.1", RouteTable: "rt1"},
			},
			natGwSR: map[string]view.StaticRoute{},
			expected: []*v1.StaticRoute{
				{Policy: "policyDst", CIDR: "1.2.3.4/32", NextHopIP: "10.0.0.1", RouteTable: "rt1"},
			},
		},
		{
			name:       "Existing NAT GW route, no new routes",
			vpcSubnets: []view.SubnetView{},
			newSRList:  []view.StaticRoute{},
			natGwSR: map[string]view.StaticRoute{
				"policyDst-0.0.0.0/0-rt1": {Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.0.254", RouteTable: "rt1"},
			},
			expected: []*v1.StaticRoute{
				{Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.0.254", RouteTable: "rt1"},
			},
		},
		{
			name: "Override existing NAT GW route with different NextHopIP",
			vpcSubnets: []view.SubnetView{
				{ObjectMeta: view.ObjectMeta{Name: "rt1"}, Spec: view.SubnetSpec{CIDRBlock: "10.0.0.0/24"}},
			},
			newSRList: []view.StaticRoute{
				{Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.0.1", RouteTable: "rt1"},
			},
			natGwSR: map[string]view.StaticRoute{
				"policyDst-0.0.0.0/0-rt1": {Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.0.254", RouteTable: "rt1"},
			},
			expected: []*v1.StaticRoute{
				{Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.0.1", RouteTable: "rt1"},
			},
		},
		{
			name: "Keep existing NAT GW route if new route has same NextHopIP",
			vpcSubnets: []view.SubnetView{
				{ObjectMeta: view.ObjectMeta{Name: "rt1"}, Spec: view.SubnetSpec{CIDRBlock: "10.0.0.0/24"}},
			},
			newSRList: []view.StaticRoute{
				{Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.0.254", RouteTable: "rt1"},
			},
			natGwSR: map[string]view.StaticRoute{
				"policyDst-0.0.0.0/0-rt1": {Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.0.254", RouteTable: "rt1"},
			},
			expected: []*v1.StaticRoute{
				{Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.0.254", RouteTable: "rt1"},
			},
		},
		{
			name: "Mixed: add new, keep same, override different",
			vpcSubnets: []view.SubnetView{
				{ObjectMeta: view.ObjectMeta{Name: "rt1"}, Spec: view.SubnetSpec{CIDRBlock: "10.0.0.0/24"}},
			},
			newSRList: []view.StaticRoute{
				{Policy: "policyDst", CIDR: "1.2.3.4/32", NextHopIP: "10.0.0.1", RouteTable: "rt1"},  // New
				{Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.0.254", RouteTable: "rt1"}, // Same
				{Policy: "policyDst", CIDR: "::/0", NextHopIP: "10.0.0.2", RouteTable: "rt1"},        // Override
			},
			natGwSR: map[string]view.StaticRoute{
				"policyDst-0.0.0.0/0-rt1": {Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.0.254", RouteTable: "rt1"},
				"policyDst-::/0-rt1":      {Policy: "policyDst", CIDR: "::/0", NextHopIP: "10.0.0.254", RouteTable: "rt1"},
			},
			expected: []*v1.StaticRoute{
				{Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.0.254", RouteTable: "rt1"},
				{Policy: "policyDst", CIDR: "1.2.3.4/32", NextHopIP: "10.0.0.1", RouteTable: "rt1"},
				{Policy: "policyDst", CIDR: "::/0", NextHopIP: "10.0.0.2", RouteTable: "rt1"},
			},
		},
		{
			name: "CIDR normalization: ::/0 should match 0:0:0:0:0:0:0:0/0",
			vpcSubnets: []view.SubnetView{
				{ObjectMeta: view.ObjectMeta{Name: "rt1"}, Spec: view.SubnetSpec{CIDRBlock: "10.0.0.0/24"}},
			},
			newSRList: []view.StaticRoute{
				{Policy: "policyDst", CIDR: "0:0:0:0:0:0:0:0/0", NextHopIP: "10.0.0.1", RouteTable: "rt1"},
			},
			natGwSR: map[string]view.StaticRoute{
				"policyDst-::/0-rt1": {Policy: "policyDst", CIDR: "::/0", NextHopIP: "10.0.0.254", RouteTable: "rt1"},
			},
			expected: []*v1.StaticRoute{
				{Policy: "policyDst", CIDR: "::/0", NextHopIP: "10.0.0.1", RouteTable: "rt1"},
			},
		},
		{
			name: "Add a new NatGateway static route with one already existing",
			vpcSubnets: []view.SubnetView{
				{ObjectMeta: view.ObjectMeta{Name: "rt2"}, Spec: view.SubnetSpec{CIDRBlock: "10.0.1.0/24"}},
			},
			newSRList: []view.StaticRoute{
				{Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.1.2", RouteTable: "rt2"},
				{Policy: "policyDst", CIDR: "::/0", NextHopIP: "10.0.1.2", RouteTable: "rt2"},
			},
			natGwSR: map[string]view.StaticRoute{
				"policyDst-0.0.0.0/0-rt1": {Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.0.1", RouteTable: "rt1"},
				"policyDst-::/0-rt1":      {Policy: "policyDst", CIDR: "::/0", NextHopIP: "10.0.0.1", RouteTable: "rt1"},
			},
			expected: []*v1.StaticRoute{
				{Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.0.1", RouteTable: "rt1"},
				{Policy: "policyDst", CIDR: "::/0", NextHopIP: "10.0.0.1", RouteTable: "rt1"},
				{Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.1.2", RouteTable: "rt2"},
				{Policy: "policyDst", CIDR: "::/0", NextHopIP: "10.0.1.2", RouteTable: "rt2"},
			},
		},
		{
			name: "Subnet not found",
			vpcSubnets: []view.SubnetView{
				{ObjectMeta: view.ObjectMeta{Name: "rt2"}, Spec: view.SubnetSpec{CIDRBlock: "10.0.1.0/24"}},
			},
			newSRList: []view.StaticRoute{
				{Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "10.0.1.2", RouteTable: "rt1"},
			},
			wantErr: true,
		},
		{
			name: "Invalid NextHopIP",
			vpcSubnets: []view.SubnetView{
				{ObjectMeta: view.ObjectMeta{Name: "rt1"}, Spec: view.SubnetSpec{CIDRBlock: "10.0.0.0/24"}},
			},
			newSRList: []view.StaticRoute{
				{Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "invalid-ip", RouteTable: "rt1"},
			},
			wantErr: true,
		},
		{
			name: "NextHopIP not in subnet CIDR",
			vpcSubnets: []view.SubnetView{
				{ObjectMeta: view.ObjectMeta{Name: "rt1"}, Spec: view.SubnetSpec{CIDRBlock: "10.0.0.0/24"}},
			},
			newSRList: []view.StaticRoute{
				{Policy: "policyDst", CIDR: "0.0.0.0/0", NextHopIP: "192.168.1.1", RouteTable: "rt1"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStaticRoutes(t.Context(), tt.vpcSubnets, tt.newSRList, tt.natGwSR)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseStaticRoutes() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			// Sort helper to compare slices
			sortRoutes := func(routes []*v1.StaticRoute) {
				slices.SortFunc(routes, func(a, b *v1.StaticRoute) int {
					if c := cmp.Compare(a.Policy, b.Policy); c != 0 {
						return c
					}
					if c := cmp.Compare(a.CIDR, b.CIDR); c != 0 {
						return c
					}
					if c := cmp.Compare(a.NextHopIP, b.NextHopIP); c != 0 {
						return c
					}
					return cmp.Compare(a.RouteTable, b.RouteTable)
				})
			}

			sortRoutes(got)
			sortRoutes(tt.expected)

			if diff := google_cmp.Diff(tt.expected, got); diff != "" {
				t.Errorf("parseStaticRoutes() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
