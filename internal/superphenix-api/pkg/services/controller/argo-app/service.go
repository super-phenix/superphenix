package argoApp

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-argo"

// API is the overridable seam for the Argo CD link endpoint; the method is the
// HTTP handler.
type API interface {
	GetArgoCdLink(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the Argo CD link route for any API.
func Module(cfg *config.Config, s API) router.Module {
	var (
		projectRead = controller.Perm(pwPermission.ProjectRead)
		argoCdRead  = controller.Perm(pwPermission.ProjectArgoCdRead)
	)
	return controller.NewControllerModule(cfg, moduleName, []router.Route{
		router.Get("/{az}/{projectId}/argo-link/{kind}/{effectiveId}", s.GetArgoCdLink, projectRead, argoCdRead),
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
