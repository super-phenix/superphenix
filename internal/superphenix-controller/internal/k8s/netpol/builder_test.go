package netpol

import (
	"errors"
	"fmt"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/models/view"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/utils"

	v1 "k8s.io/api/networking/v1"
)

func subnetViews(names ...string) []view.SubnetView {
	subnets := make([]view.SubnetView, 0, len(names))
	for _, name := range names {
		subnets = append(subnets, view.SubnetView{ObjectMeta: view.ObjectMeta{Name: name}})
	}
	return subnets
}

func subnetNames(n int) []string {
	names := make([]string, 0, n)
	for i := range n {
		names = append(names, fmt.Sprintf("spx-subnet-%d", i))
	}
	return names
}

func TestWithSubnetScope(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		subnetEIds  []string
		available   []view.SubnetView
		expectErr   bool
		expectedErr error
		expectSet   bool
		expected    string
	}{
		{
			name:        "no subnet leaves the policy unscoped",
			annotations: map[string]string{"ovn.kubernetes.io/network_policy_enforcement": "lax"},
			subnetEIds:  nil,
			available:   subnetViews("spx-subnet-a"),
		},
		{
			name:        "nil annotation map with no subnet",
			annotations: nil,
			subnetEIds:  nil,
			available:   subnetViews("spx-subnet-a"),
		},
		{
			name: "empty list clears an existing scope",
			annotations: map[string]string{
				utils.NetPolForAnnotation: "spx-project/spx-subnet-a",
			},
			subnetEIds: []string{},
			available:  subnetViews("spx-subnet-a"),
		},
		{
			name:        "nil annotation map is created",
			annotations: nil,
			subnetEIds:  []string{"spx-subnet-a"},
			available:   subnetViews("spx-subnet-a"),
			expectSet:   true,
			expected:    "spx-project/spx-subnet-a",
		},
		{
			name:        "multiple subnets",
			annotations: map[string]string{},
			subnetEIds:  []string{"spx-subnet-a", "spx-subnet-b"},
			available:   subnetViews("spx-subnet-a", "spx-subnet-b", "spx-subnet-c"),
			expectSet:   true,
			expected:    "spx-project/spx-subnet-a,spx-project/spx-subnet-b",
		},
		{
			name:        "shared subnet is accepted",
			annotations: map[string]string{},
			subnetEIds:  []string{"spx-shared-subnet"},
			available:   subnetViews("spx-subnet-a", "spx-shared-subnet"),
			expectSet:   true,
			expected:    "spx-project/spx-shared-subnet",
		},
		{
			name:        "the maximum number of subnets is accepted",
			annotations: map[string]string{},
			subnetEIds:  subnetNames(MaxSubnets),
			available:   subnetViews(subnetNames(MaxSubnets)...),
			expectSet:   true,
			expected:    utils.BuildSubnetScope("spx-project", subnetNames(MaxSubnets)),
		},
		{
			name:        "unknown subnet is rejected",
			annotations: map[string]string{},
			subnetEIds:  []string{"spx-subnet-a", "spx-unknown"},
			available:   subnetViews("spx-subnet-a"),
			expectErr:   true,
			expectedErr: ErrUnknownSubnet,
		},
		{
			name:        "no subnet available at all",
			annotations: map[string]string{},
			subnetEIds:  []string{"spx-subnet-a"},
			available:   nil,
			expectErr:   true,
			expectedErr: ErrUnknownSubnet,
		},
		{
			name:        "more than the maximum number of subnets is rejected",
			annotations: map[string]string{},
			subnetEIds:  subnetNames(MaxSubnets + 1),
			available:   subnetViews(subnetNames(MaxSubnets + 1)...),
			expectErr:   true,
			expectedErr: ErrTooManySubnets,
		},
		{
			name: "too many subnets does not clear an existing scope",
			annotations: map[string]string{
				utils.NetPolForAnnotation: "spx-project/spx-subnet-a",
			},
			subnetEIds:  subnetNames(MaxSubnets + 1),
			available:   subnetViews(subnetNames(MaxSubnets + 1)...),
			expectErr:   true,
			expectedErr: ErrTooManySubnets,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			np := &v1.NetworkPolicy{}
			np.Annotations = tt.annotations
			before := np.Annotations[utils.NetPolForAnnotation]

			err := withSubnetScope(np, "spx-project", tt.subnetEIds, tt.available)

			if tt.expectErr {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected %v, got %v", tt.expectedErr, err)
				}
				if got := np.Annotations[utils.NetPolForAnnotation]; got != before {
					t.Errorf("annotation = %q, expected it unchanged at %q", got, before)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, ok := np.Annotations[utils.NetPolForAnnotation]
			if ok != tt.expectSet {
				t.Fatalf("annotation present = %v, expected %v", ok, tt.expectSet)
			}
			if got != tt.expected {
				t.Errorf("annotation = %q, expected %q", got, tt.expected)
			}
		})
	}
}
