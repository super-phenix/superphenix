package instance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInstanceBody_GpuSerialization checks that compute.gpu survives the
// gateway's decode/re-encode with its nil / empty / set semantics intact.
func TestInstanceBody_GpuSerialization(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		wantUpdateGpu *[]InstanceGpuBody
		wantCtrlJSON  string
	}{
		{
			name:          "omitted gpu stays omitted (preserve on update)",
			payload:       `{"compute":{"cpu":2,"memory":4}}`,
			wantUpdateGpu: nil,
			wantCtrlJSON:  `{"cpu":2,"memory":4}`,
		},
		{
			name:          "null gpu is treated as omitted",
			payload:       `{"compute":{"cpu":2,"memory":4,"gpu":null}}`,
			wantUpdateGpu: nil,
			wantCtrlJSON:  `{"cpu":2,"memory":4}`,
		},
		{
			name:          "empty gpu list is forwarded (remove on update)",
			payload:       `{"compute":{"cpu":2,"memory":4,"gpu":[]}}`,
			wantUpdateGpu: &[]InstanceGpuBody{},
			wantCtrlJSON:  `{"cpu":2,"memory":4,"gpu":[]}`,
		},
		{
			name:          "gpu list is forwarded",
			payload:       `{"compute":{"cpu":2,"memory":4,"gpu":[{"device":"nvidia-rtx-pro-6000-bse"}]}}`,
			wantUpdateGpu: &[]InstanceGpuBody{{Device: "nvidia-rtx-pro-6000-bse"}},
			wantCtrlJSON:  `{"cpu":2,"memory":4,"gpu":[{"device":"nvidia-rtx-pro-6000-bse"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body UpdateInstanceBody
			require.NoError(t, json.Unmarshal([]byte(tt.payload), &body))
			assert.Equal(t, tt.wantUpdateGpu, body.Compute.Gpu)

			ctrlBody := toUpdateAzControllerBody(body, nil)
			out, err := json.Marshal(ctrlBody.Compute)
			require.NoError(t, err)
			assert.JSONEq(t, tt.wantCtrlJSON, string(out))
		})
	}
}

func TestCreateInstanceBody_GpuForwarded(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    []InstanceGpuBody
	}{
		{name: "omitted gpu", payload: `{"compute":{"cpu":2,"memory":4}}`, want: nil},
		{name: "one gpu", payload: `{"compute":{"cpu":2,"memory":4,"gpu":[{"device":"nvidia-rtx-pro-6000-bse"}]}}`, want: []InstanceGpuBody{{Device: "nvidia-rtx-pro-6000-bse"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body CreateInstanceBody
			require.NoError(t, json.Unmarshal([]byte(tt.payload), &body))

			// CreateInstance copies Compute as-is into the controller body.
			ctrlBody := CreateInstanceSpxControllerBody{Compute: body.Compute}
			assert.Equal(t, tt.want, ctrlBody.Compute.Gpu)
		})
	}
}
