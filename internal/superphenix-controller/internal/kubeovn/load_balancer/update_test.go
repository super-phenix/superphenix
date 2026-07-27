package loadBalancer

import (
	"context"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/internal/gc/resources/testhelper"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	v1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestUpdateLoadBalancer_EndpointIPv4Validation verifies only valid IPv4 endpoints are accepted.
func TestUpdateLoadBalancer_EndpointIPv4Validation(t *testing.T) {
	const (
		namespace = "prj-test"
		name      = "lb-effective"
	)

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
			// Seed an existing rule whose project label matches the namespace so the
			// CheckProjectLabel / IsEditAllowed guards pass and the endpoint loop is reached.
			existing := &v1.SwitchLBRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:   name,
					Labels: map[string]string{spxId.SpxLabelProjectID: namespace},
				},
			}
			client := testhelper.NewFakeKubeOvnClientset()
			config.KubeOvnClient = client
			if _, err := client.KubeovnV1().SwitchLBRules().Create(context.Background(), existing, metav1.CreateOptions{}); err != nil {
				t.Fatalf("failed to seed existing load balancer: %v", err)
			}

			info := &UpdateLoadBalancerInfo{
				VIP:       "198.18.0.1",
				Endpoints: tt.endpoints,
			}

			err := info.UpdateLoadBalancer(context.Background(), namespace, name)
			if tt.wantErr && err == nil {
				t.Errorf("expected an error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}
