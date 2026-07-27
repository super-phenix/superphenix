package session

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication/jwt"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication/kratos"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	apiToken "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/auth/apitoken"
)

const ModuleName = "session"

// API is the overridable seam for the session endpoints; the methods are the HTTP handlers.
type API interface {
	RetrieveAccessToken(http.ResponseWriter, *http.Request)
	Logout(http.ResponseWriter, *http.Request)
	WhoAmI(http.ResponseWriter, *http.Request)
	GenerateTokens(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the route module for any API implementation. Routes dispatch
// through the interface and are mounted under "/v1" with their auth chain.
func Module(s API) router.Module {
	var (
		jwtOrToken  = authentication.Authenticate(jwt.JwtBearerAuth, apiToken.ApiTokenAuth)
		refreshAuth = authentication.Authenticate(RefreshAuth)
		kratosAuth  = kratos.Authenticator
	)

	return router.Module{
		Name:  ModuleName,
		Mount: "/v1",
		Routes: []router.Route{
			router.Get("/session", s.RetrieveAccessToken, refreshAuth),
			router.Get("/logout", s.Logout),
			router.Get("/whoami", s.WhoAmI, jwtOrToken),
			router.Get("/session/token", s.GenerateTokens, kratosAuth),
		},
	}
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(h))
}
