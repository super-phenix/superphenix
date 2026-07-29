package health

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
)

func TestReadyz(t *testing.T) {
	// Start two TCP listeners to simulate the public and admin servers.
	publicListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start public listener: %v", err)
	}
	defer publicListener.Close()

	adminListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start admin listener: %v", err)
	}
	defer adminListener.Close()

	tests := []struct {
		name           string
		publicAddr     string
		adminEnabled   bool
		adminAddr      string
		expectedStatus int
		expectedKeys   []string
	}{
		{
			name:           "both servers up",
			publicAddr:     publicListener.Addr().String(),
			adminEnabled:   true,
			adminAddr:      adminListener.Addr().String(),
			expectedStatus: http.StatusOK,
			expectedKeys:   []string{"public", "admin"},
		},
		{
			name:           "public up, admin disabled",
			publicAddr:     publicListener.Addr().String(),
			adminEnabled:   false,
			adminAddr:      "",
			expectedStatus: http.StatusOK,
			expectedKeys:   []string{"public"},
		},
		{
			name:           "public down, admin up",
			publicAddr:     "127.0.0.1:1", // unlikely to be listening
			adminEnabled:   true,
			adminAddr:      adminListener.Addr().String(),
			expectedStatus: http.StatusServiceUnavailable,
			expectedKeys:   []string{"public", "admin"},
		},
		{
			name:           "both servers down",
			publicAddr:     "127.0.0.1:1",
			adminEnabled:   true,
			adminAddr:      "127.0.0.1:2",
			expectedStatus: http.StatusServiceUnavailable,
			expectedKeys:   []string{"public", "admin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.PublicHTTP.Address = tt.publicAddr
			cfg.AdminHTTP.Enabled = tt.adminEnabled
			cfg.AdminHTTP.Address = tt.adminAddr

			svc := New(cfg)

			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			rec := httptest.NewRecorder()

			svc.Readyz(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}

			for _, key := range tt.expectedKeys {
				if _, ok := body[key]; !ok {
					t.Errorf("expected key %q in response body, got %v", key, body)
				}
			}

			if len(body) != len(tt.expectedKeys) {
				t.Errorf("expected %d keys in response body, got %d: %v", len(tt.expectedKeys), len(body), body)
			}
		})
	}
}
