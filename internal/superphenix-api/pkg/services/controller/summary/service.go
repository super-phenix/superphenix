package summary

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-summary"

// API is the overridable seam for the summary endpoints; the methods are the
// HTTP handlers.
type API interface {
	GetProjectSummary(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the summary routes for any API.
//
// The route is gated on ProjectRead only: it answers for every product type at once, so the
// per-type read permissions cannot be expressed as middlewares and are checked in the handler.
func Module(cfg *config.Config, s API) router.Module {
	return controller.NewControllerModule(moduleName, nil, router.Group{
		Middlewares: []router.Middleware{controller.Perm(pwPermission.ProjectRead)},
		Routes: []router.Route{
			router.Get("/{projectId}/summary", s.GetProjectSummary),
		},
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
