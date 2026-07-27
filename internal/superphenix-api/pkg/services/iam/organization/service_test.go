package organization_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/organization"

	"github.com/go-chi/chi/v5"
)

// editionService embeds the default *Service (reusing Get/Update/Delete/Transfer)
// and overrides only Create, proving wrap-and-partially-override works.
type editionService struct {
	*organization.Service
	created *bool
}

func (e editionService) Create(w http.ResponseWriter, _ *http.Request) {
	*e.created = true
	w.WriteHeader(http.StatusTeapot)
}

var _ organization.API = editionService{}

// TestProvideServiceRegistersModule proves the service registers its routes under
// organization.ModuleName.
func TestProvideServiceRegistersModule(t *testing.T) {
	reg := router.New()
	organization.ProvideService(&config.Global, reg)

	// The module is overridable and removable by name.
	reg.RemoveModule(organization.ModuleName)
	root := chi.NewRouter()
	reg.Build(root)

	rr := httptest.NewRecorder()
	root.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/organization", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("after RemoveModule(%q), POST /v1/organization = %d, want 404 (module was registered under that name)", organization.ModuleName, rr.Code)
	}
}

// TestModuleDispatchesToEditionOverride proves Module wires routes through the
// Service interface, so an override is served while other endpoints use the default.
func TestModuleDispatchesToEditionOverride(t *testing.T) {
	called := false
	ee := editionService{Service: organization.New(&config.Global), created: &called}

	mod := organization.Module(ee)

	var create http.HandlerFunc
	for _, rt := range mod.Routes {
		if rt.Method == http.MethodPost && rt.Pattern == "/organization" {
			create = rt.Handler
		}
	}
	if create == nil {
		t.Fatal("POST /organization route not built by Module")
	}

	rr := httptest.NewRecorder()
	create(rr, httptest.NewRequest(http.MethodPost, "/v1/organization", nil))
	if !called || rr.Code != http.StatusTeapot {
		t.Fatalf("edition override not dispatched: called=%v code=%d, want true/418", called, rr.Code)
	}
}
