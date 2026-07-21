package adminHttp

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/metrics"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/opentelemetry/tracing"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/admin/billing"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/admin/permission"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// RegisterModules registers the global middlewares of the Admin HTTP API onto reg.
// The admin services register their own routes via their providers in pkg/server.
func RegisterModules(cfg *config.Config, reg *router.Registry) {
	reg.Use("request-id", middleware.RequestID)
	reg.Use("logger", middleware.Logger)
	reg.Use("recoverer", middleware.Recoverer)
	reg.Use("real-ip", middleware.RealIP)
	reg.Use("clean-path", middleware.CleanPath)
	reg.Use("tracing", tracing.MiddlewareHTTP)
	reg.Use("metrics", metrics.MiddlewareHTTP)
}

// BuildAdminRouter builds reg onto a fresh chi router without serving.
func BuildAdminRouter(reg *router.Registry) chi.Router {
	root := chi.NewRouter()
	reg.Build(root)
	return root
}

// wireAdminRoutes builds the admin routing for tests by calling the same providers
// directly (no wire toolchain, no DB).
func wireAdminRoutes() chi.Router {
	reg := router.New()
	RegisterModules(&config.Global, reg)

	permission.ProvideService(&config.Global, reg)
	billing.ProvideService(&config.Global, reg)

	return BuildAdminRouter(reg)
}
