package manager

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication/jwt"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/proxy"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
)

const ModuleName = "project-manager"

// API is the overridable seam for the project-manager endpoints; the methods are the HTTP handlers.
type API interface {
	ListUsers(http.ResponseWriter, *http.Request)
	ListOrganizations(http.ResponseWriter, *http.Request)
	ListProjects(http.ResponseWriter, *http.Request)
	ListProducts(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the route module for any API implementation. Routes dispatch
// through the interface and are mounted under "/v1/project-manager", enabled only
// when a super-admin whitelist is configured.
func Module(cfg *config.Config, s API) router.Module {
	return router.Module{
		Name:    ModuleName,
		Mount:   "/v1/project-manager",
		Enabled: func() bool { return len(cfg.SuperAdmins) > 0 },
		Middlewares: []router.Middleware{
			authentication.Authenticate(jwt.JwtBearerAuth),
			proxy.AddUserIdToRequestHeader,
			checkSuperAdminList,
		},
		Routes: []router.Route{
			router.Get("/users", s.ListUsers),
			router.Get("/organizations", s.ListOrganizations),
			router.Get("/projects", s.ListProjects),
			router.Get("/products", s.ListProducts),
		},
	}
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
