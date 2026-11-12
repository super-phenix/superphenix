package informers

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSwitchLBRuleVPCVIP(t *testing.T) {
	rule := func(vip string, annotations map[string]any) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"metadata": map[string]any{"name": "rule", "annotations": annotations},
			"spec":     map[string]any{"vip": vip},
		}}
	}

	tests := []struct {
		name string
		obj  any
		want []string
	}{
		{name: "pinned rule", obj: rule("198.18.0.10", map[string]any{"ovn.kubernetes.io/logical_router": "vpc-a"}), want: []string{"vpc-a|198.18.0.10"}},
		{name: "unpinned rule", obj: rule("198.18.0.10", map[string]any{}), want: []string{"*|198.18.0.10"}},
		{name: "rule without annotations", obj: rule("198.18.0.10", nil), want: []string{"*|198.18.0.10"}},
		{name: "rule without vip", obj: rule("", map[string]any{"ovn.kubernetes.io/logical_router": "vpc-a"}), want: []string{"vpc-a|"}},
		{name: "unexpected object", obj: "rule", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := switchLBRuleVPCVIP(tt.obj)
			if err != nil {
				t.Fatalf("switchLBRuleVPCVIP() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("switchLBRuleVPCVIP() = %v, want %v", got, tt.want)
			}
		})
	}
}
