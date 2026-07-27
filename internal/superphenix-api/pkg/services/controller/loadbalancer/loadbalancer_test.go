package loadbalancer

import (
	"testing"

	"github.com/super-phenix/superphenix/pkg/utils/validation"

	"github.com/stretchr/testify/assert"
)

// TestCreateLoadBalancerBody_EndpointIPv4Validation verifies the `dive,ipv4`
// validation tag on the Endpoints field: every endpoint must be a valid IPv4
// address. Empty/absent endpoints are allowed (omitempty) since Endpoints and
// Selectors are mutually exclusive.
func TestCreateLoadBalancerBody_EndpointIPv4Validation(t *testing.T) {
	v := validation.GetValidatorV1()

	tests := []struct {
		name      string
		endpoints []string
		wantValid bool
	}{
		{name: "single valid IPv4", endpoints: []string{"10.0.0.1"}, wantValid: true},
		{name: "multiple valid IPv4", endpoints: []string{"10.0.0.1", "192.168.1.2"}, wantValid: true},
		{name: "no endpoints (selector mode)", endpoints: nil, wantValid: true},
		{name: "empty slice", endpoints: []string{}, wantValid: true},
		{name: "IPv6 rejected", endpoints: []string{"fe80::1"}, wantValid: false},
		{name: "garbage rejected", endpoints: []string{"not-an-ip"}, wantValid: false},
		{name: "empty string rejected", endpoints: []string{""}, wantValid: false},
		{name: "one invalid among valid rejected", endpoints: []string{"10.0.0.1", "bad"}, wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := CreateLoadBalancerBody{}
			body.Spec.Endpoints = tt.endpoints

			err := v.ValidateStruct(body)
			if tt.wantValid {
				assert.Nil(t, err, "expected body to validate")
			} else {
				assert.NotNil(t, err, "expected validation error")
			}
		})
	}
}
