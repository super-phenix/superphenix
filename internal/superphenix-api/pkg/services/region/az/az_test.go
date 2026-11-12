package az

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	"github.com/go-chi/chi/v5"
)

func TestDefaultCloudInit(t *testing.T) {
	original := config.Global.AZs
	t.Cleanup(func() { config.Global.AZs = original })
	config.Global.AZs = map[string]config.AZConfig{
		"public":  {DefaultCloudInit: "#public"},
		"private": {DefaultCloudInit: "#private", Whitelist: []string{"org-allowed"}},
	}

	tests := []struct {
		name       string
		orga       string
		az         string
		wantStatus int
		wantConfig string
	}{
		{name: "known AZ", orga: "org", az: "public", wantStatus: http.StatusOK, wantConfig: "#public"},
		{name: "whitelisted organization", orga: "org-allowed", az: "private", wantStatus: http.StatusOK, wantConfig: "#private"},
		{name: "organization not whitelisted", orga: "org", az: "private", wantStatus: http.StatusNotFound},
		{name: "unknown AZ", orga: "org", az: "missing", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Get("/organization/{orgaId}/az/{az}/default-cloud-init", New(&config.Global).DefaultCloudInit)

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/organization/"+tt.orga+"/az/"+tt.az+"/default-cloud-init", nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}
			var got model.APIAzDefaultCloudInit
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("invalid body: %v", err)
			}
			if got.Config != tt.wantConfig {
				t.Fatalf("config = %q, want %q", got.Config, tt.wantConfig)
			}
		})
	}
}
