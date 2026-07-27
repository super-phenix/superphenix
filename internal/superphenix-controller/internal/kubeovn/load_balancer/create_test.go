package loadBalancer

import (
	"context"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/testhelper"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
)

// TestCreateLoadBalancer_EndpointIPv4Validation verifies only valid IPv4 endpoints are accepted.
func TestCreateLoadBalancer_EndpointIPv4Validation(t *testing.T) {
	tests := []struct {
		name      string
		endpoints []string
		wantErr   bool
	}{
		{name: "valid IPv4 endpoint", endpoints: []string{"10.0.0.1"}, wantErr: false},
		{name: "multiple valid IPv4 endpoints", endpoints: []string{"10.0.0.1", "192.168.1.2"}, wantErr: false},
		{name: "IPv6 endpoint rejected", endpoints: []string{"fe80::1"}, wantErr: true},
		{name: "IPv6 loopback rejected", endpoints: []string{"::1"}, wantErr: true},
		{name: "garbage endpoint rejected", endpoints: []string{"not-an-ip"}, wantErr: true},
		{name: "empty endpoint rejected", endpoints: []string{""}, wantErr: true},
		{name: "one invalid among valid rejected", endpoints: []string{"10.0.0.1", "bad"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.KubeOvnClient = testhelper.NewFakeKubeOvnClientset()

			info := &CreateLoadBalancerInfo{
				VIP:       "198.18.0.1",
				Endpoints: tt.endpoints,
			}
			info.OrgId = "org"
			info.ProjectId = "project"
			info.ResourceLocalId = "lb"
			info.ResourceEffectiveId = "lb-effective"

			err := info.CreateLoadBalancer(context.Background())
			if tt.wantErr && err == nil {
				t.Errorf("expected an error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}
