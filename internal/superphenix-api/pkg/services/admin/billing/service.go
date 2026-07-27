package billing

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/adminHttp/authentication"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
)

const ModuleName = "billing"

// API is the overridable seam for the billing endpoints; the methods are the HTTP handlers.
type API interface {
	GetProjectName(http.ResponseWriter, *http.Request)
	GetOrgaName(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the route module for any API implementation, mounted under
// "/billing" with the billing secret auth.
func Module(cfg *config.Config, s API) router.Module {
	return router.Module{
		Name:        ModuleName,
		Mount:       "/billing",
		Middlewares: []router.Middleware{authentication.SecretAuth(cfg.BillingHTTP.AuthSecret)},
		Routes: []router.Route{
			router.Get("/project/{projectId}", s.GetProjectName),
			router.Get("/organization/{orgaId}", s.GetOrgaName),
		},
	}
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
