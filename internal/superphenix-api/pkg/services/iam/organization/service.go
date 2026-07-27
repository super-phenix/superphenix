package organization

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authorization/permify"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication/jwt"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	apiToken "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/auth/apitoken"

	pwPerm "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

// ModuleName is the registry key for the organization routes, so they can be overridden or removed.
const ModuleName = "organization"

// API is the overridable seam for the organization endpoints; the methods are the HTTP handlers.
type API interface {
	Create(http.ResponseWriter, *http.Request)
	Get(http.ResponseWriter, *http.Request)
	Update(http.ResponseWriter, *http.Request)
	Delete(http.ResponseWriter, *http.Request)
	Transfer(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

// Module builds the route module for any API implementation. Routes dispatch
// through the interface and are mounted under "/v1" with their per-route
// auth/permission chain.
func Module(s API) router.Module {
	var (
		jwtOrToken = authentication.Authenticate(jwt.JwtBearerAuth, apiToken.ApiTokenAuth)
		orgaRead   = permify.CheckPermission(pwPerm.OrganizationRead)
		orgaWrite  = permify.CheckPermission(pwPerm.OrganizationWrite)
	)

	return router.Module{
		Name:  ModuleName,
		Mount: "/v1",
		Routes: []router.Route{
			router.Post("/organization", s.Create, jwtOrToken),
			router.Get("/organization/{orgaId}", s.Get, jwtOrToken, orgaRead),
			router.Post("/organization/{orgaId}", s.Update, jwtOrToken, orgaRead, orgaWrite),
			router.Delete("/organization/{orgaId}", s.Delete, jwtOrToken, orgaRead, orgaWrite),
			router.Post("/organization/{orgaId}/transfer", s.Transfer, jwtOrToken, orgaRead, orgaWrite),
		},
	}
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(h))
}
