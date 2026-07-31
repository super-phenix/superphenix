package api

import (
	"net/http"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/api"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/authentication"
	configApi "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/config"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/gc"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/k8s"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/kubeovn"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/kubevirt"
	customMw "github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/middleware"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/paas"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/api/utils"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/metrics"
	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/opentelemetry/tracing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
	httpSwagger "github.com/swaggo/http-swagger"
)

// LaunchEndpoint
//
//	@title						Superphenix Controller
//	@version					0.1
//	@description				Superphenix Controller HTTP API
//
//	@contact.name				API Support
//	@contact.url				https://superphenix.net
//	@contact.email				contact@superphenix.net
//
//	@BasePath					/
//
//	@securityDefinitions.apiKey	UserID
//	@in							header
//	@name						X-User-Id
//	@description				User ID accessing to the controller.
func LaunchEndpoint(address string) {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RealIP)
	router.Use(middleware.CleanPath)
	router.Use(utils.AddUserToContext)
	router.Use(customMw.RequestLogger)
	router.Use(tracing.MiddlewareHTTP)
	router.Use(metrics.MiddlewareHTTP)

	// Heartbeat middleware returns if the router is alive
	router.Use(middleware.Heartbeat("/health"))

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	router.Use(middleware.Timeout(60 * time.Second))

	router.Group(func(r chi.Router) {
		setupDocumentation(r)
	})

	router.Route("/{orgId}/{projectId}", func(r chi.Router) {
		r.Use(authentication.BearerAuth())

		r.Get("/mark", gc.MarkForDeletion)
		r.Get("/storage-class", configApi.GetStorageClass)
		r.Get("/vm-type", configApi.GetVMClusterPreference)
		r.Get("/vm-type/{name}", configApi.GetVMClusterPreferenceByName)
		r.Get("/vm-type/{name}/advanced-options", configApi.GetVMClusterPreferenceAdvancedOptions)
		r.Get("/kaas-config", configApi.GetKaaSConfig)
		r.Get("/s3-config", configApi.GetS3Config)
		r.Get("/container-disks", configApi.GetContainerDiskCatalog)

		//// COMPUTE ////
		kubevirt.VMEndpoint(r)
		kubevirt.SerialEndpoint(r)
		kubevirt.VNCEndpoint(r)
		kubevirt.InstanceSnapshotEndpoint(r)

		//// STORAGE ////
		kubevirt.DiskEndpoint(r)
		kubevirt.SnapshotEndpoint(r)
		paas.BaasEndpoint(r)
		k8s.BucketEndpoint(r)

		//// NETWORK ////
		kubeovn.VPCEndpoint(r)
		kubeovn.SubnetEndpoint(r)
		kubeovn.EipEndpoint(r)
		kubeovn.LoadBalancerEndpoint(r)
		k8s.NetPolEndpoint(r)

		//// PaaS ////
		paas.KaaSEndpoint(r)

		//// OTHERS ////
		k8s.SSHEndpoint(r)
	})

	log.Info().Msgf("Http Server starting at %s", address)
	if err := http.ListenAndServe(address, router); err != nil {
		log.Fatal().Err(err).Msg("failed to start Http Server")
	}
}

// setupDocumentation exposes the documentation for the API
func setupDocumentation(router chi.Router) {
	api.SwaggerInfo.Host = config.Global.Swagger.BaseURL
	api.SwaggerInfo.Schemes = []string{"http", "https"}

	router.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("./swagger/doc.json")),
	)
}
