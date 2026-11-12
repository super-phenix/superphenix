package loadBalancer

import (
	"context"
	"testing"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildSpec(t *testing.T) {
	tests := []struct {
		name      string
		vip       string
		selectors []string
		endpoints []string
		wantErr   bool
	}{
		{name: "valid IPv4 endpoint", vip: testVIP, endpoints: []string{"10.0.0.1"}, wantErr: false},
		{name: "multiple valid IPv4 endpoints", vip: testVIP, endpoints: []string{"10.0.0.1", "192.168.1.2"}, wantErr: false},
		{name: "valid selector", vip: testVIP, selectors: []string{"app:web"}, wantErr: false},
		{name: "invalid selector rejected", vip: testVIP, selectors: []string{"app"}, wantErr: true},
		{name: "IPv6 endpoint rejected", vip: testVIP, endpoints: []string{"fe80::1"}, wantErr: true},
		{name: "IPv6 loopback rejected", vip: testVIP, endpoints: []string{"::1"}, wantErr: true},
		{name: "garbage endpoint rejected", vip: testVIP, endpoints: []string{"not-an-ip"}, wantErr: true},
		{name: "empty endpoint rejected", vip: testVIP, endpoints: []string{""}, wantErr: true},
		{name: "one invalid among valid rejected", vip: testVIP, endpoints: []string{"10.0.0.1", "bad"}, wantErr: true},
		{name: "invalid vip rejected", vip: "bad", endpoints: []string{"10.0.0.1"}, wantErr: true},
		{name: "vip outside the allowed CIDR rejected", vip: "10.0.0.10", endpoints: []string{"10.0.0.1"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildSpec(tt.vip, project, tt.selectors, tt.endpoints, nil)
			if (err != nil) != tt.wantErr {
				t.Fatalf("buildSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.IsBadRequest(err) {
				t.Errorf("expected a BadRequest, got %v", err)
			}
		})
	}
}

func TestCreateLoadBalancer(t *testing.T) {
	tests := []struct {
		name      string
		fixture   fixture
		subnet    string
		endpoints []string
		selectors []string
		wantErr   bool
		wantVPC   string
	}{
		{name: "missing subnet rejected", endpoints: []string{"10.10.0.5"}, wantErr: true},
		{name: "unknown subnet rejected", subnet: "spx-unknown", endpoints: []string{"10.10.0.5"}, wantErr: true},
		{name: "subnet of another project rejected", subnet: subnetOther, endpoints: []string{"10.30.0.5"}, wantErr: true},
		{name: "subnet in the default VPC rejected", subnet: subnetCluster, endpoints: []string{"10.40.0.5"}, wantErr: true},
		{name: "own subnet pins the rule", subnet: subnetA, endpoints: []string{"10.10.0.5"}, wantVPC: vpcA, wantErr: false},
		{name: "shared subnet pins the rule", subnet: subnetShared, endpoints: []string{"10.20.0.5"}, wantVPC: vpcB, wantErr: false},
		{name: "endpoint outside the subnet rejected", subnet: subnetA, endpoints: []string{"10.11.0.5"}, wantErr: true},
		{name: "one endpoint outside the subnet rejected", subnet: subnetA, endpoints: []string{"10.10.0.5", "10.99.0.5"}, wantErr: true},
		{name: "selector accepted", subnet: subnetA, selectors: []string{"app:web"}, wantVPC: vpcA, wantErr: false},
		{
			name:      "vip used by a rule of the same VPC rejected",
			fixture:   fixture{rules: []*v1.SwitchLBRule{ruleObj("other", project, testVIP, pinned(vpcA, subnetA2))}},
			subnet:    subnetA,
			endpoints: []string{"10.10.0.5"},
			wantErr:   true,
		},
		{
			name:      "vip used by a rule of another VPC accepted",
			fixture:   fixture{rules: []*v1.SwitchLBRule{ruleObj("other", otherProject, testVIP, pinned(vpcB, subnetOther))}},
			subnet:    subnetA,
			endpoints: []string{"10.10.0.5"},
			wantVPC:   vpcA,
			wantErr:   false,
		},
		{
			name: "vip used by an unpinned rule resolved in the same VPC rejected",
			fixture: fixture{
				rules:    []*v1.SwitchLBRule{ruleObj("other", project, testVIP, nil)},
				services: []*corev1.Service{ruleService("other", project, vpcA)},
			},
			subnet:    subnetA,
			endpoints: []string{"10.10.0.5"},
			wantErr:   true,
		},
		{
			name: "vip used by an unpinned rule resolved in another VPC accepted",
			fixture: fixture{
				rules:    []*v1.SwitchLBRule{ruleObj("other", otherProject, testVIP, nil)},
				services: []*corev1.Service{ruleService("other", otherProject, defaultVPC)},
			},
			subnet:    subnetA,
			endpoints: []string{"10.10.0.5"},
			wantVPC:   vpcA,
			wantErr:   false,
		},
		{
			name:      "vip used by an unpinned rule of unknown VPC rejected",
			fixture:   fixture{rules: []*v1.SwitchLBRule{ruleObj("other", otherProject, testVIP, nil)}},
			subnet:    subnetA,
			endpoints: []string{"10.10.0.5"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := setup(t, tt.fixture)

			info := &CreateLoadBalancerInfo{
				VIP:       testVIP,
				SubnetEId: tt.subnet,
				Selectors: tt.selectors,
				Endpoints: tt.endpoints,
			}
			info.OrgId = "org"
			info.ProjectId = projectUUID
			info.ResourceLocalId = "lb"
			info.ResourceEffectiveId = "lb-effective"

			err := info.CreateLoadBalancer(context.Background())
			if (err != nil) != tt.wantErr {
				t.Fatalf("CreateLoadBalancer() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if !errors.IsBadRequest(err) {
					t.Errorf("expected a BadRequest, got %v", err)
				}
				return
			}

			rule, err := client.KubeovnV1().SwitchLBRules().Get(context.Background(), info.GetResourceEffectiveID(), metav1.GetOptions{})
			if err != nil {
				t.Fatalf("rule not created: %v", err)
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
