package utils

import (
	"reflect"
	"testing"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"
)

func TestFilterLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected map[string]string
	}{
		{
			name: "Keep Spx labels",
			labels: map[string]string{
				spxId.SpxLabelPrefix + "my-label": "value1",
				"other-label":                     "value2",
			},
			expected: map[string]string{
				spxId.SpxLabelPrefix + "my-label": "value1",
			},
		},
		{
			name: "Keep Custom VM labels",
			labels: map[string]string{
				CustomLabelPrefix + "my-vm-label": "value1",
				"random-label":                    "value2",
			},
			expected: map[string]string{
				CustomLabelPrefix + "my-vm-label": "value1",
			},
		},
		{
			name: "Keep allowed labels",
			labels: map[string]string{
				"velero.io/backup-name":  "backup1",
				"velero.io/restore-name": "restore1",
				"kubernetes.io/name":     "k8s",
			},
			expected: map[string]string{
				"velero.io/backup-name":  "backup1",
				"velero.io/restore-name": "restore1",
			},
		},
		{
			name: "Exclude specific labels",
			labels: map[string]string{
				spxId.SpxLabelPrefix + "keep-me":        "yes",
				"superphenix.net/ignoreNetworkPolicies": "true",
				"superphenix.net/workloadClass":         "app",
			},
			expected: map[string]string{
				spxId.SpxLabelPrefix + "keep-me": "yes",
			},
		},
		{
			name:     "Empty labels",
			labels:   map[string]string{},
			expected: map[string]string{},
		},
		{
			name:     "Nil labels",
			labels:   nil,
			expected: map[string]string{},
		},
		{
			name: "Mix of labels",
			labels: map[string]string{
				spxId.SpxLabelPrefix + "owner":  "admin",
				CustomLabelPrefix + "env":       "prod",
				"velero.io/backup-name":         "daily",
				"superphenix.net/workloadClass": "system",
				"unrelated":                     "data",
			},
			expected: map[string]string{
				spxId.SpxLabelPrefix + "owner": "admin",
				CustomLabelPrefix + "env":      "prod",
				"velero.io/backup-name":        "daily",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterLabels(tt.labels)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("FilterLabels() = %v, want %v", got, tt.expected)
			}
		})
	}
}
