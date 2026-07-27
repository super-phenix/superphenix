package permission

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/adminHttp/authentication"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
)

const ModuleName = "admin-permission"

// API is the overridable seam for the admin permission endpoints; the methods are the HTTP handlers.
type API interface {
	UpdateDefaultSchema(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the route module for any API implementation, mounted under
// "/admin" with the admin secret auth.
func Module(cfg *config.Config, s API) router.Module {
	return router.Module{
		Name:        ModuleName,
		Mount:       "/admin",
		Middlewares: []router.Middleware{authentication.SecretAuth(cfg.AdminHTTP.AuthSecret)},
		Routes: []router.Route{
			router.Post("/permission/update-schema", s.UpdateDefaultSchema),
		},
	}
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
