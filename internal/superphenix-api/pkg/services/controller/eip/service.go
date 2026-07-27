package eip

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-eip"

// API is the overridable seam for the EIP endpoints; the methods are the
// HTTP handlers.
type API interface {
	ListEips(http.ResponseWriter, *http.Request)
	ListAZEips(http.ResponseWriter, *http.Request)
	GetEip(http.ResponseWriter, *http.Request)
	CreateEip(http.ResponseWriter, *http.Request)
	UpdateEip(http.ResponseWriter, *http.Request)
	DeleteEip(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the EIP routes for any API.
func Module(cfg *config.Config, s API) router.Module {
	var (
		eipRead  = controller.Perm(pwPermission.ProjectEipRead)
		eipWrite = controller.Perm(pwPermission.ProjectEipWrite)
		quota    = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(cfg, moduleName, nil, router.Group{
		Middlewares: []router.Middleware{eipRead},
		Routes: []router.Route{
			router.Get("/{projectId}/eip", s.ListEips),
			router.Get("/{az}/{projectId}/eip", s.ListAZEips),
			router.Get("/{az}/{projectId}/eip/{effectiveId}", s.GetEip),
		},
		Groups: []router.Group{{
			Middlewares: []router.Middleware{eipWrite},
			Routes: []router.Route{
				router.Post("/{az}/{projectId}/eip", s.CreateEip, quota),
				router.Post("/{az}/{projectId}/eip/{effectiveId}", s.UpdateEip),
				router.Delete("/{az}/{projectId}/eip/{effectiveId}", s.DeleteEip),
			},
		}},
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
