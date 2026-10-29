package kaas

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestUnstructuredToMachineDeployment_NodeRoleLabels(t *testing.T) {
	tests := []struct {
		name           string
		templateLabels map[string]any
		want           map[string]string
	}{
		{
			name: "keeps the node role label",
			templateLabels: map[string]any{
				"node-role.kubernetes.io/gpu":         "gpu",
				"superphenix.net/resourceEffectiveID": "spx-1",
			},
			want: map[string]string{"node-role.kubernetes.io/gpu": "gpu"},
		},
		{
			name:           "drops other template labels",
			templateLabels: map[string]any{"superphenix.net/generated": "true"},
			want:           nil,
		},
		{
			name: "no template labels",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := map[string]any{}
			if tt.templateLabels != nil {
				template["metadata"] = map[string]any{"labels": tt.templateLabels}
			}
			item := unstructured.Unstructured{Object: map[string]any{
				"spec": map[string]any{"template": template},
			}}

			got := unstructuredToMachineDeployment(item).Spec.Template.Labels
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("template labels = %v, want %v", got, tt.want)
			}
		})
	}
}
