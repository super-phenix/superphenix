package publicHttp

import (
	"net/http"

	apiToken "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/auth/apitoken"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/auth/session"
	argoApp "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app"
	baasctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/baas"
	diskctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/disk"
	eipctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/eip"
	firewallctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/firewall"
	instancectrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/instance"
	kaasctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/kaas"
	loadbalancerctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/loadbalancer"
	metadatactrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/metadata"
	snapshotctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/snapshot"
	sshctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/ssh"
	subnetctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/subnet"
	vmsnapshotctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/vmsnapshot"
	vpcctrl "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/vpc"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/group"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/membership"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/organization"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/permission"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/user"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/project/manager"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/project/project"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/region/az"

	"github.com/super-phenix/superphenix/internal/superphenix-api/api"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/metrics"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/opentelemetry/tracing"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger"
)

// RegisterModules registers the global middlewares and built-in modules onto reg.
// The organization service registers its own routes via its provider in pkg/server.
func RegisterModules(cfg *config.Config, reg *router.Registry) {
	reg.Use("request-id", middleware.RequestID)
	reg.Use("logger", middleware.Logger)
	reg.Use("recoverer", middleware.Recoverer)
	reg.Use("real-ip", middleware.RealIP)
	reg.Use("clean-path", middleware.CleanPath)
	reg.Use("tracing", tracing.MiddlewareHTTP)
	reg.Use("metrics", metrics.MiddlewareHTTP)
	reg.Use("cors", sessionCorsMiddleware(cfg))
	reg.Use("heartbeat", middleware.Heartbeat(cfg.PublicHTTP.HealthEndpoint))

	reg.Register(swaggerModule(cfg))
}

// BuildPublicRouter builds reg onto a fresh chi router without serving.
func BuildPublicRouter(reg *router.Registry) chi.Router {
	root := chi.NewRouter()
	reg.Build(root)
	return root
}

// wirePublicRoutes assembles the full public routing for tests by calling the same
// providers directly (no wire toolchain, no DB).
func wirePublicRoutes() chi.Router {
	reg := router.New()
	RegisterModules(&config.Global, reg)

	organization.ProvideService(&config.Global, reg)
	session.ProvideService(&config.Global, reg)
	apiToken.ProvideService(&config.Global, reg)
	az.ProvideService(&config.Global, reg)
	user.ProvideService(&config.Global, reg)
	group.ProvideService(&config.Global, reg)
	membership.ProvideService(&config.Global, reg)
	permission.ProvideService(&config.Global, reg)
	project.ProvideService(&config.Global, reg)
	manager.ProvideService(&config.Global, reg)

	instancectrl.ProvideService(&config.Global, reg)
	vmsnapshotctrl.ProvideService(&config.Global, reg)
	diskctrl.ProvideService(&config.Global, reg)
	snapshotctrl.ProvideService(&config.Global, reg)
	baasctrl.ProvideService(&config.Global, reg)
	vpcctrl.ProvideService(&config.Global, reg)
	subnetctrl.ProvideService(&config.Global, reg)
	eipctrl.ProvideService(&config.Global, reg)
	loadbalancerctrl.ProvideService(&config.Global, reg)
	firewallctrl.ProvideService(&config.Global, reg)
	sshctrl.ProvideService(&config.Global, reg)
	kaasctrl.ProvideService(&config.Global, reg)
	metadatactrl.ProvideService(&config.Global, reg)
	argoApp.ProvideService(&config.Global, reg)

	return BuildPublicRouter(reg)
}

// swaggerModule exposes the API documentation.
func swaggerModule(cfg *config.Config) router.Module {
	api.SwaggerInfo.Host = cfg.Swagger.BaseURL
	api.SwaggerInfo.Schemes = []string{"http", "https"}

	return router.Module{
		Name: "swagger",
		Routes: []router.Route{
			{
				Method:  http.MethodGet,
				Pattern: "/swagger/*",
				Handler: httpSwagger.Handler(httpSwagger.URL("./swagger/doc.json")),
			},
		},
	}
}

func sessionCorsMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   cfg.Session.Cors.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	})
}
