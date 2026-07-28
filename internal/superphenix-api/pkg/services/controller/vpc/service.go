package vpc

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-vpc"

// API is the overridable seam for the VPC endpoints; the methods are the
// HTTP handlers.
type API interface {
	ListVPCs(http.ResponseWriter, *http.Request)
	ListAZVPCs(http.ResponseWriter, *http.Request)
	GetVPC(http.ResponseWriter, *http.Request)
	CreateVPC(http.ResponseWriter, *http.Request)
	UpdateVPC(http.ResponseWriter, *http.Request)
	DeleteVPC(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the VPC routes for any API.
func Module(cfg *config.Config, s API) router.Module {
	var (
		vpcRead  = controller.Perm(pwPermission.ProjectVPCRead)
		vpcWrite = controller.Perm(pwPermission.ProjectVPCWrite)
		quota    = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(moduleName, nil, router.Group{
		Middlewares: []router.Middleware{vpcRead},
		Routes: []router.Route{
			router.Get("/{projectId}/vpc", s.ListVPCs),
			router.Get("/{az}/{projectId}/vpc", s.ListAZVPCs),
			router.Get("/{az}/{projectId}/vpc/{effectiveId}", s.GetVPC),
		},
		Groups: []router.Group{{
			Middlewares: []router.Middleware{vpcWrite},
			Routes: []router.Route{
				router.Post("/{az}/{projectId}/vpc", s.CreateVPC, quota),
				router.Post("/{az}/{projectId}/vpc/{effectiveId}", s.UpdateVPC),
				router.Delete("/{az}/{projectId}/vpc/{effectiveId}", s.DeleteVPC),
			},
		}},
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
