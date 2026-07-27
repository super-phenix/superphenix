package az

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
)

const HealthModuleName = "admin-az-health"

// API is the overridable seam for the admin AZ endpoints; the methods are the HTTP handlers.
// AZ is now sourced from configuration, so only the read-only health endpoint remains
// (the DB-backed write endpoints were removed upstream).
type API interface {
	FullHealth(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// HealthModule is the unauthenticated AZ health route.
func HealthModule(cfg *config.Config, s API) router.Module {
	return router.Module{
		Name: HealthModuleName,
		Routes: []router.Route{
			router.Get(cfg.AdminHTTP.HealthEndpoint+"/full", s.FullHealth),
		},
	}
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(HealthModule(cfg, h))
}
