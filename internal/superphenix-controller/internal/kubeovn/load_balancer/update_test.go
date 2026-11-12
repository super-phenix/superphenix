package loadBalancer

import (
	"context"
	"testing"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdateLoadBalancer(t *testing.T) {
	const name = "lb-effective"

	tests := []struct {
		name         string
		fixture      fixture
		subnet       string
		vip          string
		endpoints    []string
		wantErr      bool
		wantNotFound bool
		wantVPC      string
	}{
		{
			name:      "pinned rule updated in place",
			fixture:   fixture{rules: []*v1.SwitchLBRule{ruleObj(name, project, testVIP, pinned(vpcA, subnetA))}},
			subnet:    subnetA,
			vip:       "198.18.0.11",
			endpoints: []string{"10.10.0.6"},
			wantVPC:   vpcA,
			wantErr:   false,
		},
		{
			name:      "pinned rule keeps its own vip",
			fixture:   fixture{rules: []*v1.SwitchLBRule{ruleObj(name, project, testVIP, pinned(vpcA, subnetA))}},
			subnet:    subnetA,
			vip:       testVIP,
			endpoints: []string{"10.10.0.6"},
			wantVPC:   vpcA,
			wantErr:   false,
		},
		{
			name:      "subnet of a pinned rule can't change",
			fixture:   fixture{rules: []*v1.SwitchLBRule{ruleObj(name, project, testVIP, pinned(vpcA, subnetA))}},
			subnet:    subnetA2,
			vip:       testVIP,
			endpoints: []string{"10.11.0.6"},
			wantErr:   true,
		},
		{
			name:    "missing subnet rejected",
			fixture: fixture{rules: []*v1.SwitchLBRule{ruleObj(name, project, testVIP, pinned(vpcA, subnetA))}},
			vip:     testVIP,
			wantErr: true,
		},
		{
			name:      "endpoint outside the subnet rejected",
			fixture:   fixture{rules: []*v1.SwitchLBRule{ruleObj(name, project, testVIP, pinned(vpcA, subnetA))}},
			subnet:    subnetA,
			vip:       testVIP,
			endpoints: []string{"10.11.0.6"},
			wantErr:   true,
		},
		{
			name: "vip used by another rule of the VPC rejected",
			fixture: fixture{rules: []*v1.SwitchLBRule{
				ruleObj(name, project, testVIP, pinned(vpcA, subnetA)),
				ruleObj("other", project, "198.18.0.11", pinned(vpcA, subnetA2)),
			}},
			subnet:    subnetA,
			vip:       "198.18.0.11",
			endpoints: []string{"10.10.0.6"},
			wantErr:   true,
		},
		{
			name:      "unpinned rule rejected",
			fixture:   fixture{rules: []*v1.SwitchLBRule{ruleObj(name, project, testVIP, nil)}},
			subnet:    subnetA,
			vip:       testVIP,
			endpoints: []string{"10.10.0.6"},
			wantErr:   true,
		},
		{
			name:         "rule of another project not found",
			fixture:      fixture{rules: []*v1.SwitchLBRule{ruleObj(name, otherProject, testVIP, pinned(vpcB, subnetOther))}},
			subnet:       subnetA,
			vip:          testVIP,
			endpoints:    []string{"10.10.0.6"},
			wantErr:      true,
			wantNotFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := setup(t, tt.fixture)

			info := &UpdateLoadBalancerInfo{VIP: tt.vip, SubnetEId: tt.subnet, Endpoints: tt.endpoints}
			err := info.UpdateLoadBalancer(context.Background(), project, name)
			if (err != nil) != tt.wantErr {
				t.Fatalf("UpdateLoadBalancer() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.wantNotFound && !errors.IsNotFound(err) || !tt.wantNotFound && !errors.IsBadRequest(err) {
					t.Errorf("unexpected error type: %v", err)
				}
				return
			}

			rule, err := client.KubeovnV1().SwitchLBRules().Get(context.Background(), name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("rule not found after update: %v", err)
			}
			if rule.Spec.Vip != tt.vip {
				t.Errorf("vip = %q, want %q", rule.Spec.Vip, tt.vip)
			}
			if got := rule.Annotations[logicalRouterAnnotation]; got != tt.wantVPC {
				t.Errorf("logical_router = %q, want %q", got, tt.wantVPC)
			}
			if got := rule.Annotations[logicalSwitchAnnotation]; got != tt.subnet {
				t.Errorf("logical_switch = %q, want %q", got, tt.subnet)
			}
		})
	}
}
