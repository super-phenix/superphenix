package healthHttp

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/health"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// RegisterModules registers the global middlewares of the Health HTTP API onto reg.
func RegisterModules(_ *config.Config, reg *router.Registry) {
	reg.Use("request-id", middleware.RequestID)
	reg.Use("logger", middleware.Logger)
	reg.Use("recoverer", middleware.Recoverer)
}

// BuildHealthRouter builds reg onto a fresh chi router without serving.
func BuildHealthRouter(reg *router.Registry) chi.Router {
	root := chi.NewRouter()
	reg.Build(root)
	return root
}

// wireHealthRoutes builds the health routing for tests by calling the same
// providers directly (no wire toolchain, no DB).
func wireHealthRoutes() chi.Router {
	reg := router.New()
	RegisterModules(&config.Global, reg)

	health.ProvideService(&config.Global, reg)

	return BuildHealthRouter(reg)
}
