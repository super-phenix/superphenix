package validation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateComputeSize(t *testing.T) {
	tests := []struct {
		name        string
		cpu         int
		memory      string
		want        string
		expectError bool
	}{
		{name: "minimum bounds", cpu: MinCPU, memory: "128Mi", want: "128Mi", expectError: false},
		{name: "maximum bounds", cpu: MaxComputeValue, memory: "999999Gi", want: "999999Gi", expectError: false},
		{name: "non power of two", cpu: 8, memory: "24Gi", want: "24Gi", expectError: false},
		{name: "fractional gibibytes", cpu: 2, memory: "1.5Gi", want: "1536Mi", expectError: false},
		{name: "mebibytes not whole gibibytes", cpu: 2, memory: "2500Mi", want: "2500Mi", expectError: false},
		{name: "mebibytes canonicalised", cpu: 2, memory: "1024Mi", want: "1Gi", expectError: false},
		{name: "bytes", cpu: 2, memory: "1073741824", want: "1Gi", expectError: false},
		{name: "memory explicit plus sign", cpu: 2, memory: "+4Gi", want: "4Gi", expectError: false},
		{name: "memory kibibytes canonicalised", cpu: 2, memory: "4194304Ki", want: "4Gi", expectError: false},
		{name: "memory milli whole bytes", cpu: 2, memory: "4398046511104000m", want: "4Ti", expectError: false},
		{name: "cpu zero", cpu: 0, memory: "4Gi", expectError: true},
		{name: "cpu negative", cpu: -1, memory: "4Gi", expectError: true},
		{name: "cpu above max", cpu: MaxComputeValue + 1, memory: "4Gi", expectError: true},
		{name: "memory below min", cpu: 2, memory: "127Mi", expectError: true},
		{name: "memory zero", cpu: 2, memory: "0", expectError: true},
		{name: "memory negative", cpu: 2, memory: "-1Gi", expectError: true},
		{name: "memory above max", cpu: 2, memory: "1000000Gi", expectError: true},
		{name: "memory not whole mebibytes", cpu: 2, memory: "1.2Gi", expectError: true},
		{name: "memory fractional bytes", cpu: 2, memory: "1048575.5", expectError: true},
		{name: "memory decimal unit", cpu: 2, memory: "2G", expectError: true},
		{name: "memory unparseable", cpu: 2, memory: "abc", expectError: true},
		{name: "memory empty", cpu: 2, memory: "", expectError: true},
		{name: "memory lowercase unit", cpu: 2, memory: "4gi", expectError: true},
		{name: "memory byte suffix", cpu: 2, memory: "4GiB", expectError: true},
		{name: "memory leading space", cpu: 2, memory: " 4Gi", expectError: true},
		{name: "memory trailing space", cpu: 2, memory: "4Gi ", expectError: true},
		{name: "memory hexadecimal", cpu: 2, memory: "0x10", expectError: true},
		{name: "memory kilo decimal unit", cpu: 2, memory: "4k", expectError: true},
		{name: "memory exponent not whole mebibytes", cpu: 2, memory: "1e9", expectError: true},
		{name: "memory milli not whole bytes", cpu: 2, memory: "4398046511104m", expectError: true},
		{name: "memory exbibytes", cpu: 2, memory: "8Ei", expectError: true},
		{name: "memory max int64 bytes", cpu: 2, memory: "9223372036854775807", expectError: true},
		{name: "memory huge exponent", cpu: 2, memory: "1e9999999", expectError: true},
		{name: "memory exponent whole mebibytes", cpu: 2, memory: "134217728e0", expectError: true},
		{name: "memory uppercase exponent", cpu: 2, memory: "1E9", expectError: true},
		{name: "memory too long", cpu: 2, memory: "1" + strings.Repeat("0", 64) + "Mi", expectError: true},
		{name: "memory fractional trailing zeros", cpu: 2, memory: "1.5000Gi", want: "1536Mi", expectError: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateComputeSize(tt.cpu, tt.memory)
			if tt.expectError {
				assert.ErrorIs(t, err, ErrInvalidComputeSize)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got.String())
		})
	}
}
