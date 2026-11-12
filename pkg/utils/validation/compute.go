package validation

import (
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Compute sizing bounds shared by VM instances and KaaS node groups.
const (
	MinCPU = 1
	// MaxComputeValue is the largest value Helm renders as a plain integer.
	MaxComputeValue = 999_999

	mebibyte = 1 << 20
	// maxMemoryLength bounds the input passed to resource.ParseQuantity.
	maxMemoryLength = 64
)

var (
	MinMemory = resource.MustParse("128Mi")
	MaxMemory = resource.MustParse(fmt.Sprintf("%dGi", MaxComputeValue))
)

// ErrInvalidComputeSize is wrapped by every error returned from ValidateComputeSize.
var ErrInvalidComputeSize = errors.New("invalid compute size")

// ValidateComputeSize checks cpu and the memory quantity, and returns the memory in canonical form.
func ValidateComputeSize(cpu int, memory string) (resource.Quantity, error) {
	if cpu < MinCPU || cpu > MaxComputeValue {
		return resource.Quantity{}, fmt.Errorf("%w: cpu value (%d) must be between %d and %d",
			ErrInvalidComputeSize, cpu, MinCPU, MaxComputeValue)
	}

	invalidMemory := fmt.Errorf("%w: memory (%q) must be a whole number of MiB between %s and %s",
		ErrInvalidComputeSize, memory, MinMemory.String(), MaxMemory.String())

	// ParseQuantity time grows with the exponent value, so exponents are rejected before parsing.
	if len(memory) > maxMemoryLength || strings.ContainsAny(memory, "eE") {
		return resource.Quantity{}, invalidMemory
	}

	q, err := resource.ParseQuantity(memory)
	if err != nil ||
		q.Cmp(MinMemory) < 0 || q.Cmp(MaxMemory) > 0 ||
		q.CmpInt64(q.Value()) != 0 || q.Value()%mebibyte != 0 {
		return resource.Quantity{}, invalidMemory
	}

	return *resource.NewQuantity(q.Value(), resource.BinarySI), nil
}
