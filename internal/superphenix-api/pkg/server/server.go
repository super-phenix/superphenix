// Package server assembles and serves the public and admin HTTP APIs. It is the
// composition root: it wires the registry, the infrastructure bootstrap, and the
// service Providers together, then builds the routers.
package server

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/adminHttp"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/healthHttp"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/app"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// Server holds the assembled public HTTP router.
type Server struct {
	public chi.Router
}

// InitializeServer builds the public Server with the default service set.
func InitializeServer(cfg *config.Config) (*Server, error) {
	return InitializeServerWith(cfg, DefaultProviders())
}

// InitializeServerWith builds the public Server from the given Providers. An edition
// passes a DefaultProviders() copy with selected fields overridden.
func InitializeServerWith(cfg *config.Config, p Providers) (*Server, error) {
	if err := app.ProvideInfra(cfg); err != nil {
		return nil, err
	}
	reg := router.New()
	publicHttp.RegisterModules(cfg, reg)
	p.registerPublic(cfg, reg)
	return &Server{public: publicHttp.BuildPublicRouter(reg)}, nil
}

// Run serves the public API and blocks.
func (s *Server) Run(addr string) error {
	log.Info().Str("address", addr).Msg("Starting Public HTTP API")
	return http.ListenAndServe(addr, s.public)
}

// AdminServer holds the assembled admin HTTP router.
type AdminServer struct {
	admin chi.Router
}

// InitializeAdminServer builds the admin Server with the default service set.
func InitializeAdminServer(cfg *config.Config) (*AdminServer, error) {
	return InitializeAdminServerWith(cfg, DefaultProviders())
}

// InitializeAdminServerWith builds the admin Server from the given Providers. The
// admin endpoints are DB-backed, so it initialises the shared infrastructure too
// (idempotent: a no-op if the public server already did it).
func InitializeAdminServerWith(cfg *config.Config, p Providers) (*AdminServer, error) {
	if err := app.ProvideInfra(cfg); err != nil {
		return nil, err
	}
	reg := router.New()
	adminHttp.RegisterModules(cfg, reg)
	p.registerAdmin(cfg, reg)
	return &AdminServer{admin: adminHttp.BuildAdminRouter(reg)}, nil
}

// Run serves the admin API and blocks.
func (s *AdminServer) Run(addr string) error {
	log.Info().Str("address", addr).Msg("Starting Admin HTTP API")
	return http.ListenAndServe(addr, s.admin)
}

// HealthServer holds the assembled health HTTP router.
type HealthServer struct {
	health chi.Router
}

// InitializeHealthServer builds the health Server with the default service set.
func InitializeHealthServer(cfg *config.Config) (*HealthServer, error) {
	return InitializeHealthServerWith(cfg, DefaultProviders())
}

// InitializeHealthServerWith builds the health Server from the given Providers.
func InitializeHealthServerWith(cfg *config.Config, p Providers) (*HealthServer, error) {
	reg := router.New()
	healthHttp.RegisterModules(cfg, reg)
	p.registerHealth(cfg, reg)
	return &HealthServer{health: healthHttp.BuildHealthRouter(reg)}, nil
}

// Run serves the health API and blocks.
func (s *HealthServer) Run(addr string) error {
	log.Info().Str("address", addr).Msg("Starting Health HTTP API")
	return http.ListenAndServe(addr, s.health)
}
