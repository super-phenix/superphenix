package utils

import (
	"reflect"
	"testing"
)

func TestBuildSubnetScope(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		subnetEIds []string
		expected   string
	}{
		{
			name:       "no subnet",
			namespace:  "spx-project",
			subnetEIds: []string{},
			expected:   "",
		},
		{
			name:       "nil subnets",
			namespace:  "spx-project",
			subnetEIds: nil,
			expected:   "",
		},
		{
			name:       "single subnet",
			namespace:  "spx-project",
			subnetEIds: []string{"spx-subnet-a"},
			expected:   "spx-project/spx-subnet-a",
		},
		{
			name:       "multiple subnets keep their order",
			namespace:  "spx-project",
			subnetEIds: []string{"spx-subnet-b", "spx-subnet-a"},
			expected:   "spx-project/spx-subnet-b,spx-project/spx-subnet-a",
		},
		{
			name:       "duplicates are dropped",
			namespace:  "spx-project",
			subnetEIds: []string{"spx-subnet-a", "spx-subnet-a", "spx-subnet-b"},
			expected:   "spx-project/spx-subnet-a,spx-project/spx-subnet-b",
		},
		{
			name:       "empty entries are skipped",
			namespace:  "spx-project",
			subnetEIds: []string{"", "spx-subnet-a"},
			expected:   "spx-project/spx-subnet-a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSubnetScope(tt.namespace, tt.subnetEIds)
			if got != tt.expected {
				t.Errorf("BuildSubnetScope() = %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestParseSubnetScope(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    []string
	}{
		{
			name:        "no annotation",
			annotations: map[string]string{"ovn.kubernetes.io/network_policy_enforcement": "lax"},
			expected:    nil,
		},
		{
			name:        "nil annotations",
			annotations: nil,
			expected:    nil,
		},
		{
			name:        "empty annotation",
			annotations: map[string]string{NetPolForAnnotation: ""},
			expected:    nil,
		},
		{
			name:        "single reference",
			annotations: map[string]string{NetPolForAnnotation: "spx-project/spx-subnet-a"},
			expected:    []string{"spx-subnet-a"},
		},
		{
			name:        "multiple references",
			annotations: map[string]string{NetPolForAnnotation: "spx-project/spx-subnet-a,spx-project/spx-subnet-b"},
			expected:    []string{"spx-subnet-a", "spx-subnet-b"},
		},
		{
			name:        "spaces and trailing separator",
			annotations: map[string]string{NetPolForAnnotation: "spx-project/spx-subnet-a, spx-project/spx-subnet-b,"},
			expected:    []string{"spx-subnet-a", "spx-subnet-b"},
		},
		{
			name:        "reference without a namespace",
			annotations: map[string]string{NetPolForAnnotation: "spx-subnet-a"},
			expected:    []string{"spx-subnet-a"},
		},
		{
			name:        "only separators",
			annotations: map[string]string{NetPolForAnnotation: ",,"},
			expected:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSubnetScope(tt.annotations)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParseSubnetScope() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestSubnetScopeRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		subnetEIds []string
	}{
		{name: "single", subnetEIds: []string{"spx-subnet-a"}},
		{name: "multiple", subnetEIds: []string{"spx-subnet-a", "spx-subnet-b", "spx-subnet-c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := map[string]string{NetPolForAnnotation: BuildSubnetScope("spx-project", tt.subnetEIds)}
			got := ParseSubnetScope(annotations)
			if !reflect.DeepEqual(got, tt.subnetEIds) {
				t.Errorf("round trip = %v, expected %v", got, tt.subnetEIds)
			}
		})
	}
}
