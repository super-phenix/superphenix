package az

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authorization/permify"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication/jwt"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	apiToken "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/auth/apitoken"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "az"

// API is the overridable seam for the AZ endpoints; the methods are the HTTP handlers.
type API interface {
	ListAZs(http.ResponseWriter, *http.Request)
	StatusCheck(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the route module for any API implementation. Routes dispatch
// through the interface and are mounted under "/v1" with their auth/permission chain.
func Module(s API) router.Module {
	var (
		jwtOrToken = authentication.Authenticate(jwt.JwtBearerAuth, apiToken.ApiTokenAuth)
		orgaRead   = permify.CheckPermission(pwPermission.OrganizationRead)
	)

	return router.Module{
		Name:  moduleName,
		Mount: "/v1",
		Routes: []router.Route{
			router.Get("/organization/{orgaId}/az-list", s.ListAZs, jwtOrToken, orgaRead),
			router.Get("/organization/{orgaId}/az-status", s.StatusCheck, jwtOrToken, orgaRead),
		},
	}
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(h))
}
