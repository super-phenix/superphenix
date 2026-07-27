package metadata

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"
)

const moduleName = "spx-controller-metadata"

// API is the overridable seam for the cluster-metadata endpoints; every method is
// a SimpleRedirect passthrough to the matching AZ controller.
type API interface {
	StorageClass(http.ResponseWriter, *http.Request)
	VMType(http.ResponseWriter, *http.Request)
	VMTypeByName(http.ResponseWriter, *http.Request)
	VMTypeAdvancedOptions(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the cluster-metadata routes for any API.
// These routes carry no per-route permission beyond the shared chain.
func Module(cfg *config.Config, s API) router.Module {
	return controller.NewControllerModule(cfg, moduleName, []router.Route{
		router.Get("/{az}/{projectId}/storage-class", s.StorageClass),
		router.Get("/{az}/{projectId}/vm-type", s.VMType),
		router.Get("/{az}/{projectId}/vm-type/{name}", s.VMTypeByName),
		router.Get("/{az}/{projectId}/vm-type/{name}/advanced-options", s.VMTypeAdvancedOptions),
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
