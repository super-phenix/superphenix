package loadbalancer

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
			body.Spec.SubnetEId = "spx-subnet"
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

// TestLoadBalancerBody_SubnetRequired verifies create and update bodies require the subnet.
func TestLoadBalancerBody_SubnetRequired(t *testing.T) {
	v := validation.GetValidatorV1()

	tests := []struct {
		name      string
		subnetEId string
		wantValid bool
	}{
		{name: "subnet set", subnetEId: "spx-subnet", wantValid: true},
		{name: "subnet missing", subnetEId: "", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			create := CreateLoadBalancerBody{}
			create.Spec.SubnetEId = tt.subnetEId
			update := UpdateLoadBalancerBody{}
			update.Spec.SubnetEId = tt.subnetEId

			for _, body := range []any{create, update} {
				err := v.ValidateStruct(body)
				if tt.wantValid {
					assert.Nil(t, err, "expected body to validate")
				} else {
					assert.NotNil(t, err, "expected validation error")
				}
			}
		})
	}
}

// TestHandleControllerError verifies a controller 400 is forwarded and other failures use the failure code.
func TestHandleControllerError(t *testing.T) {
	tests := []struct {
		name           string
		controllerCode int
		wantCode       int
	}{
		{name: "bad request forwarded", controllerCode: http.StatusBadRequest, wantCode: http.StatusBadRequest},
		{name: "conflict uses the failure code", controllerCode: http.StatusConflict, wantCode: http.StatusInternalServerError},
		{name: "server error uses the failure code", controllerCode: http.StatusInternalServerError, wantCode: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
			resp := &http.Response{
				StatusCode: tt.controllerCode,
				Status:     http.StatusText(tt.controllerCode),
				Body:       io.NopCloser(strings.NewReader(`{"message":"reason"}`)),
				Request:    r,
			}
			w := httptest.NewRecorder()

			handleControllerError(w, r, resp, http.StatusInternalServerError, "failure")
			assert.Equal(t, tt.wantCode, w.Code)
		})
	}
}
