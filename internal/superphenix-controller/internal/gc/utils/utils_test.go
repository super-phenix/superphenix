package utils

import (
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	"testing"
	"time"
)

type fakeResource struct {
	name   string
	labels map[string]string
}

func (f *fakeResource) GetName() string              { return f.name }
func (f *fakeResource) GetLabels() map[string]string { return f.labels }

func TestParseTimestamp(t *testing.T) {
	labelKey := "superphenix.net/markedForDeletion"
	config.Global.GarbageCollection.LabelMarkKey = labelKey

	pastTimestamp := time.Now().Add(-1 * time.Hour).Format(TimestampFormat)
	futureTimestamp := time.Now().Add(1 * time.Hour).Format(TimestampFormat)

	tests := []struct {
		name    string
		labels  map[string]string
		want    bool
		wantErr bool
	}{
		{
			name:    "past timestamp returns true",
			labels:  map[string]string{labelKey: pastTimestamp},
			want:    true,
			wantErr: false,
		},
		{
			name:    "future timestamp returns false",
			labels:  map[string]string{labelKey: futureTimestamp},
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
			name:    "invalid timestamp returns error",
			labels:  map[string]string{labelKey: "not-a-timestamp"},
			want:    false,
			wantErr: true,
		},
		{
			name:    "empty timestamp returns error",
			labels:  map[string]string{labelKey: ""},
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

func TestTransform(t *testing.T) {
	type Source struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	type Target struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name    string
		input   Source
		want    Target
		wantErr bool
	}{
		{
			name:    "successful transform",
			input:   Source{Name: "test", Value: 42},
			want:    Target{Name: "test", Value: 42},
			wantErr: false,
		},
		{
			name:    "empty struct transform",
			input:   Source{},
			want:    Target{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Transform[Source, Target](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Transform() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.Name != tt.want.Name || got.Value != tt.want.Value {
				t.Errorf("Transform() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
