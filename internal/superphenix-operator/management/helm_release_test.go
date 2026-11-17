package management

import (
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
)

func TestHelmReleaseMatches(t *testing.T) {
	ch := &chart.Chart{Metadata: &chart.Metadata{Name: "argo-cd", Version: "7.7.12"}}
	vals := map[string]interface{}{"server": map[string]interface{}{"replicas": float64(1)}}

	tests := []struct {
		name string
		rel  *release.Release
		ch   *chart.Chart
		vals map[string]interface{}
		want bool
	}{
		{
			name: "matching deployed chart and values",
			rel: &release.Release{
				Info:   &release.Info{Status: release.StatusDeployed},
				Chart:  &chart.Chart{Metadata: &chart.Metadata{Name: "argo-cd", Version: "7.7.12"}},
				Config: map[string]interface{}{"server": map[string]interface{}{"replicas": float64(1)}},
			},
			ch:   ch,
			vals: vals,
			want: true,
		},
		{
			name: "nil and empty values match when deployed",
			rel: &release.Release{
				Info:   &release.Info{Status: release.StatusDeployed},
				Chart:  &chart.Chart{Metadata: &chart.Metadata{Name: "argo-cd", Version: "7.7.12"}},
				Config: nil,
			},
			ch:   ch,
			vals: nil,
			want: true,
		},
		{
			name: "matching failed release does not skip upgrade",
			rel: &release.Release{
				Info:   &release.Info{Status: release.StatusFailed},
				Chart:  &chart.Chart{Metadata: &chart.Metadata{Name: "argo-cd", Version: "7.7.12"}},
				Config: map[string]interface{}{"server": map[string]interface{}{"replicas": float64(1)}},
			},
			ch:   ch,
			vals: vals,
			want: false,
		},
		{
			name: "chart version changed",
			rel: &release.Release{
				Info:   &release.Info{Status: release.StatusDeployed},
				Chart:  &chart.Chart{Metadata: &chart.Metadata{Name: "argo-cd", Version: "7.7.11"}},
				Config: vals,
			},
			ch:   ch,
			vals: vals,
			want: false,
		},
		{
			name: "values changed",
			rel: &release.Release{
				Info:   &release.Info{Status: release.StatusDeployed},
				Chart:  &chart.Chart{Metadata: &chart.Metadata{Name: "argo-cd", Version: "7.7.12"}},
				Config: map[string]interface{}{"server": map[string]interface{}{"replicas": float64(2)}},
			},
			ch:   ch,
			vals: vals,
			want: false,
		},
		{
			name: "missing release chart metadata",
			rel:  &release.Release{Info: &release.Info{Status: release.StatusDeployed}},
			ch:   ch,
			vals: vals,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := helmReleaseMatches(tt.rel, tt.ch, tt.vals); got != tt.want {
				t.Fatalf("helmReleaseMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}
