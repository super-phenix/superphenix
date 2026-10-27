package ctrlutils

import "testing"

func TestIsGenerated(t *testing.T) {
	withLabels := func(labels map[string]any) map[string]any {
		return map[string]any{"eip": map[string]any{"metadata": map[string]any{"labels": labels}}}
	}

	tests := []struct {
		name string
		item map[string]any
		want bool
	}{
		{name: "generated", item: withLabels(map[string]any{generatedLabel: "true"}), want: true},
		{name: "not generated", item: withLabels(map[string]any{generatedLabel: "false"}), want: false},
		{name: "no label", item: withLabels(map[string]any{}), want: false},
		{name: "no resource", item: map[string]any{}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGenerated(tt.item, "eip"); got != tt.want {
				t.Errorf("IsGenerated() = %v, want %v", got, tt.want)
			}
		})
	}
}
