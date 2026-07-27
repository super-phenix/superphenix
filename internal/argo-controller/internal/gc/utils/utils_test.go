package utils

import (
	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/config"
	"testing"
	"time"
)

type fakeResource struct {
	name   string
	labels map[string]string
}

func (f *fakeResource) GetName() string {
	return f.name
}

func (f *fakeResource) GetLabels() map[string]string {
	return f.labels
}

func TestParseTimestamp(t *testing.T) {
	config.Global.GarbageCollection.LabelMarkKey = "superphenix.net/markedForDeletion"

	tests := []struct {
		name    string
		labels  map[string]string
		want    bool
		wantErr bool
	}{
		{
			name: "timestamp in the past returns true",
			labels: map[string]string{
				"superphenix.net/markedForDeletion": time.Now().Add(-1 * time.Hour).Format(TimestampFormat),
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "timestamp in the future returns false",
			labels: map[string]string{
				"superphenix.net/markedForDeletion": time.Now().Add(1 * time.Hour).Format(TimestampFormat),
			},
			want:    false,
			wantErr: false,
		},
		{
			name:    "missing timestamp returns error",
			labels:  map[string]string{},
			want:    false,
			wantErr: true,
		},
		{
			name: "invalid timestamp format returns error",
			labels: map[string]string{
				"superphenix.net/markedForDeletion": "not-a-timestamp",
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "empty timestamp value returns error",
			labels: map[string]string{
				"superphenix.net/markedForDeletion": "",
			},
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeResource{name: "test", labels: tt.labels}
			got, err := ParseTimestamp(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseTimestamp() = %v, want %v", got, tt.want)
			}
		})
	}
}
