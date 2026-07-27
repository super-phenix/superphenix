package api

import (
	"context"
	"net/http"

	"github.com/super-phenix/superphenix/internal/argo-controller/api"
	v1 "github.com/super-phenix/superphenix/internal/argo-controller/pkg/api/v1"
	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/config"
	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/metrics"
	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/opentelemetry/tracing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
	httpSwagger "github.com/swaggo/http-swagger"
)

// StartAPI
//
//	@title									SPX Argo Controller
//	@version								0.1
//	@description							SPX Argo Controller HTTP API
//
//	@contact.name							API Support
//	@contact.url							https://superphenix.net
//	@contact.email							contact@superphenix.net
//
//	@BasePath								/
//
//	@securityDefinitions.apiKey				Bearer
//	@in										header
//	@name									Authorization
//	@description							Auth Token to have access to Argo Ctrl. "Bearer [Auth Token]"
//	@scope.OrganizationRead					Grants read access to organization
//	@scope.OrganizationWrite				Grants write access to organization
//	@scope.OrganizationIAMRead				Grants read access to organization IAM
//	@scope.OrganizationIAMWrite				Grants write access to organization IAM
//	@scope.OrganizationBillingRead			Grants read access to organization billing
//	@scope.OrganizationBillingWrite			Grants write access to organization billing
//	@scope.OrganizationProjectManagement	Grants project management access in organization
//	@scope.ProjectInstanceRead				Grants read access to instance in project
//	@scope.ProjectInstanceTerminal			Grants terminal access to instance in project
//	@scope.ProjectInstanceControl			Grants control (start, stop) access to instance in project
//	@scope.ProjectInstanceWrite				Grants write access to instance in project
//	@scope.ProjectSnapshotRead				Grants read access to Snapshot and Instance Snapshot in project
//	@scope.ProjectSnapshotWrite				Grants write access to Snapshot and Instance Snapshot in project
//	@scope.ProjectDiskRead					Grants read access to Disk in project
//	@scope.ProjectDiskWrite					Grants write access to Disk in project
//	@scope.ProjectVPCRead					Grants read access to VPC in project
//	@scope.ProjectVPCWrite					Grants write access to VPC in project
//	@scope.ProjectSubnetRead				Grants read access to Subnet in project
//	@scope.ProjectSubnetWrite				Grants write access to Subnet in project
//	@scope.ProjectEipRead					Grants read access to EIP in project
//	@scope.ProjectEipWrite					Grants write access to EIP in project
//	@scope.ProjectLBRead					Grants read access to LoadBalancer in project
//	@scope.ProjectLBWrite					Grants write access to LoadBalancer in project
//	@scope.ProjectSSHRead					Grants read access to SSH in project
//	@scope.ProjectSSHWrite					Grants write access to SSH in project
func StartAPI() {
	log.Info().Str("address", config.Global.Http.Address).Msg("Starting HTTP API")
	startHTTP(config.Global.Http.Address, config.Global.Http.HealthEndpoint)
}

var versions = make(map[string]func(r chi.Router))

// startHTTP initializes the main router and middlewares for the Public HTTP API
func startHTTP(bindAddress, heartbeatEndpoint string) {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RealIP)
	router.Use(middleware.CleanPath)
	router.Use(tracing.MiddlewareHTTP)
	router.Use(metrics.MiddlewareHTTP)

	router.Use(middleware.Heartbeat(heartbeatEndpoint))

	setupDocumentation(router)

	registerVersions()
	setupVersions(router)

	// Launch the router
	if err := http.ListenAndServe(bindAddress, router); err != nil {
		log.Fatal().Err(err).Msg("Failed to start Public HTTP endpoint")
	}
}

// registerVersions registers all supported versions of the API
func registerVersions() {
	registerRouterVersion("v1", v1.V1Router)
}

// setupVersions creates a sub-router for each version defined in the "version" map
// and injects the version in the context of the HTTP request through a middleware
func setupVersions(router *chi.Mux) {
	for version, versionRouter := range versions {
		router.Route("/"+version, func(r chi.Router) {
			r.Use(apiVersionCtx(version))
			versionRouter(r)
		})
	}
}

// apiVersionCtx adds a context variable to the request containing the version of the accessed route
func apiVersionCtx(version string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), "api.version", version))
			next.ServeHTTP(w, r)
		})
	}
}

// registerRouterVersion adds a new router for a specific API version
func registerRouterVersion(version string, router func(r chi.Router)) {
	versions[version] = router
}

// setupDocumentation exposes the documentation for the API
func setupDocumentation(router chi.Router) {
	api.SwaggerInfo.Host = config.Global.Swagger.BaseURL
	api.SwaggerInfo.Schemes = []string{"http", "https"}

	router.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("./swagger/doc.json")),
	)
}
