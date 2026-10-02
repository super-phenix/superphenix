package kaas

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/azversion"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"

	"github.com/google/uuid"
)

// fakeResolver returns fixed AZ versions, or err when set.
type fakeResolver struct {
	versions map[string]string
	err      error
}

func (f fakeResolver) Version(_ context.Context, az config.AZConfig) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.versions[az.Code], nil
}

func testKaasConfig() *config.Config {
	cfg := &config.Config{}
	cfg.ProductsConfig.ArgoApp.Kubernetes.Versions = map[string]config.KaasVersionProfile{
		"0.0.0": {
			Repo:         config.RepoArgoAppConfig{RepoURL: "ghcr.io/super-phenix/charts", Chart: "sfs-kaas", TargetRevision: "0.3.8"},
			KubeVersions: []config.KubeVersionConfig{{Version: "v1.34.8"}},
		},
		"0.7.0": {
			Repo:         config.RepoArgoAppConfig{RepoURL: "ghcr.io/super-phenix/charts", Chart: "sfs-kaas", TargetRevision: "0.7.1"},
			KubeVersions: []config.KubeVersionConfig{{Version: "v1.36.3"}, {Version: "v1.35.5"}},
		},
	}
	return cfg
}

func TestKaasProfile(t *testing.T) {
	cfg := testKaasConfig()
	cfg.ProductsConfig.ArgoApp.Kubernetes.Versions = map[string]config.KaasVersionProfile{
		"0.7.0": cfg.ProductsConfig.ArgoApp.Kubernetes.Versions["0.7.0"],
	}

	tests := []struct {
		name         string
		resolver     fakeResolver
		wantOk       bool
		wantRevision string
		wantStatus   int
		wantContext  map[string]string
	}{
		{
			name:         "profile of the AZ version",
			resolver:     fakeResolver{versions: map[string]string{"az1": "0.8.2"}},
			wantOk:       true,
			wantRevision: "0.7.1",
		},
		{
			name:        "AZ version below every profile",
			resolver:    fakeResolver{versions: map[string]string{"az1": "0.6.3"}},
			wantStatus:  http.StatusConflict,
			wantContext: map[string]string{"az": "az1", "spxVersion": "0.6.3"},
		},
		{
			name:        "AZ version unreadable",
			resolver:    fakeResolver{err: azversion.ErrUnknownVersion},
			wantStatus:  http.StatusServiceUnavailable,
			wantContext: map[string]string{"az": "az1", "reason": azversion.ErrUnknownVersion.Error()},
		},
		{
			name:       "AZ version not a semver",
			resolver:   fakeResolver{versions: map[string]string{"az1": "main"}},
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(cfg, nil, tt.resolver)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

			profile, ok := h.kaasProfile(w, r, config.AZConfig{Code: "az1"})
			if ok != tt.wantOk {
				t.Fatalf("kaasProfile() ok = %v, want %v", ok, tt.wantOk)
			}
			if ok {
				if profile.Repo.TargetRevision != tt.wantRevision {
					t.Errorf("kaasProfile() revision = %q, want %q", profile.Repo.TargetRevision, tt.wantRevision)
				}
				return
			}
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			var body httpError.ErrorBody
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("error body: %v", err)
			}
			for k, v := range tt.wantContext {
				if body.Context[k] != v {
					t.Errorf("context[%q] = %q, want %q", k, body.Context[k], v)
				}
			}
		})
	}
}

func TestOptionalKaasProfile(t *testing.T) {
	tests := []struct {
		name     string
		resolver fakeResolver
		wantNil  bool
	}{
		{name: "resolved", resolver: fakeResolver{versions: map[string]string{"az1": "0.7.0"}}},
		{name: "unknown version", resolver: fakeResolver{err: errors.New("boom")}, wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(testKaasConfig(), nil, tt.resolver)
			got := h.optionalKaasProfile(context.Background(), config.AZConfig{Code: "az1"})
			if (got == nil) != tt.wantNil {
				t.Errorf("optionalKaasProfile() = %+v, want nil %v", got, tt.wantNil)
			}
		})
	}
}

func TestCombineListResult(t *testing.T) {
	profile := testKaasConfig().ProductsConfig.ArgoApp.Kubernetes.Versions["0.7.0"]

	product := model.Product{EffectiveID: "spx-eid", ProductName: "cluster", CodeAZ: "az1"}
	product.ID = uuid.New()
	azEntry := func(chart, kubeVersion string) map[string]interface{} {
		entry := map[string]interface{}{"id": product.ID.String(), "eid": product.EffectiveID, "gitops": "false", "cluster": nil}
		if chart != "" {
			entry["chart"] = chart
		}
		if kubeVersion != "" {
			entry["kubeVersion"] = kubeVersion
		}
		return entry
	}
	flag := func(b bool) *bool { return &b }

	known := map[string]*config.KaasVersionProfile{"az1": &profile}
	unknown := map[string]*config.KaasVersionProfile{"az1": nil}

	tests := []struct {
		name     string
		az       map[string][]any
		profiles map[string]*config.KaasVersionProfile
		want     *bool
	}{
		{name: "up to date", az: map[string][]any{"az1": {azEntry("sfs-kaas-0.7.1", "v1.36.3")}}, profiles: known, want: flag(false)},
		{name: "outdated", az: map[string][]any{"az1": {azEntry("sfs-kaas-0.3.8", "v1.36.3")}}, profiles: known, want: flag(true)},
		{name: "unknown AZ profile", az: map[string][]any{"az1": {azEntry("sfs-kaas-0.3.8", "v1.36.3")}}, profiles: unknown, want: nil},
		{name: "controller without chart", az: map[string][]any{"az1": {azEntry("", "")}}, profiles: known, want: nil},
		{name: "db only", az: map[string][]any{}, profiles: known, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := map[uuid.UUID]bool{product.ID: false}
			got := combineListResult(tt.az, []model.Product{product}, check, tt.profiles)
			if len(got) != 1 {
				t.Fatalf("combineListResult() returned %d entries, want 1", len(got))
			}
			switch {
			case tt.want == nil && got[0].Outdated != nil:
				t.Errorf("Outdated = %v, want nil", *got[0].Outdated)
			case tt.want != nil && (got[0].Outdated == nil || *got[0].Outdated != *tt.want):
				t.Errorf("Outdated = %v, want %v", got[0].Outdated, *tt.want)
			}
		})
	}
}
