package az

import (
	"errors"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
)

func TestGetByCode(t *testing.T) {
	config.Global.AZs = []config.AZConfig{
		{Code: "az1", Name: "AZ One", ControllerUrl: "http://az1"},
		{Code: "az2", Name: "AZ Two", ControllerUrl: "http://az2"},
		{Code: "az3", Name: "AZ Three", ControllerUrl: "http://az3", Whitelist: []string{"org-1", "org-2"}},
	}

	tests := []struct {
		name    string
		code    string
		orgaId  string
		wantErr bool
	}{
		{name: "existing az without whitelist", code: "az1", orgaId: "org-99", wantErr: false},
		{name: "another existing az without whitelist", code: "az2", orgaId: "org-99", wantErr: false},
		{name: "non-existing az", code: "az-unknown", orgaId: "org-1", wantErr: true},
		{name: "whitelisted az with allowed org", code: "az3", orgaId: "org-1", wantErr: false},
		{name: "whitelisted az with another allowed org", code: "az3", orgaId: "org-2", wantErr: false},
		{name: "whitelisted az with denied org", code: "az3", orgaId: "org-99", wantErr: true},
		{name: "whitelisted az with empty orgaId", code: "az3", orgaId: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetByCode(tt.code, tt.orgaId)
			if tt.wantErr {
				if !errors.Is(err, ErrAZNotFound) {
					t.Errorf("expected ErrAZNotFound, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result.Code != tt.code {
					t.Errorf("expected code %s, got %s", tt.code, result.Code)
				}
			}
		})
	}
}

func TestFindAll(t *testing.T) {
	config.Global.AZs = []config.AZConfig{
		{Code: "az1", Name: "AZ One"},
		{Code: "az2", Name: "AZ Two", Whitelist: []string{"org-1", "org-2"}},
		{Code: "az3", Name: "AZ Three", Whitelist: []string{"org-3"}},
	}

	tests := []struct {
		name      string
		orgaId    string
		wantCodes []string
	}{
		{name: "org in whitelist gets public + whitelisted", orgaId: "org-1", wantCodes: []string{"az1", "az2"}},
		{name: "org not in any whitelist gets only public", orgaId: "org-99", wantCodes: []string{"az1"}},
		{name: "org in az3 whitelist", orgaId: "org-3", wantCodes: []string{"az1", "az3"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindAll(tt.orgaId)
			if len(result) != len(tt.wantCodes) {
				t.Fatalf("expected %d AZs, got %d", len(tt.wantCodes), len(result))
			}
			for i, code := range tt.wantCodes {
				if result[i].Code != code {
					t.Errorf("expected code %s at index %d, got %s", code, i, result[i].Code)
				}
			}
		})
	}
}
