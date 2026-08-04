package baas

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/app"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"
	argoApp "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-baas"

// API is the overridable seam for the BaaS endpoints; the methods are the HTTP
// handlers.
type API interface {
	ListBaaS(http.ResponseWriter, *http.Request)
	GetBaaS(http.ResponseWriter, *http.Request)
	CreateBaaS(http.ResponseWriter, *http.Request)
	GetForUpdateBaaS(http.ResponseWriter, *http.Request)
	UpdateBaaS(http.ResponseWriter, *http.Request)
	DeleteBaaS(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg  *config.Config
	argo argoApp.Client
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects; the Argo client is
// injected so tests can fake it, and may be nil when no cluster is reachable.
func New(cfg *config.Config, argoClient argoApp.Client) *Service {
	return &Service{cfg: cfg, argo: argoClient}
}

// Module builds the BaaS routes for any API.
func Module(cfg *config.Config, s API) router.Module {
	var (
		baasRead  = controller.Perm(pwPermission.ProjectBaaSRead)
		baasWrite = controller.Perm(pwPermission.ProjectBaaSWrite)
		quota     = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(moduleName, nil, router.Group{
		Middlewares: []router.Middleware{baasRead},
		Routes: []router.Route{
			router.Get("/{projectId}/baas", s.ListBaaS),
			router.Get("/{az}/{projectId}/baas/{effectiveId}", s.GetBaaS),
		},
		Groups: []router.Group{{
			Middlewares: []router.Middleware{baasWrite},
			Routes: []router.Route{
				router.Post("/{az}/{projectId}/baas", s.CreateBaaS, quota),
				router.Get("/{az}/{projectId}/baas/{effectiveId}/app", s.GetForUpdateBaaS),
				router.Post("/{az}/{projectId}/baas/{effectiveId}", s.UpdateBaaS),
				router.Delete("/{az}/{projectId}/baas/{effectiveId}", s.DeleteBaaS),
			},
		}},
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg, app.ProvideArgo(cfg))
	reg.Register(Module(cfg, h))
}
