package subnet

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-subnet"

// API is the overridable seam for the subnet endpoints; the methods
// are the HTTP handlers (named handlers and SimpleRedirect passthroughs).
type API interface {
	ListSubnets(http.ResponseWriter, *http.Request)
	ListAZSubnets(http.ResponseWriter, *http.Request)
	GetSubnet(http.ResponseWriter, *http.Request)
	CreateSubnet(http.ResponseWriter, *http.Request)
	UpdateSubnet(http.ResponseWriter, *http.Request)
	DeleteSubnet(http.ResponseWriter, *http.Request)
	HasEip(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the subnet routes for any API.
func Module(cfg *config.Config, s API) router.Module {
	var (
		subnetRead  = controller.Perm(pwPermission.ProjectSubnetRead)
		subnetWrite = controller.Perm(pwPermission.ProjectSubnetWrite)
		eipRead     = controller.Perm(pwPermission.ProjectEipRead)
		quota       = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(moduleName, nil, router.Group{
		Middlewares: []router.Middleware{subnetRead},
		Routes: []router.Route{
			router.Get("/{projectId}/subnet", s.ListSubnets),
			router.Get("/{az}/{projectId}/subnet", s.ListAZSubnets),
			router.Get("/{az}/{projectId}/subnet/{effectiveId}", s.GetSubnet),
			router.Get("/{az}/{projectId}/subnet/{effectiveId}/has-eip", s.HasEip, eipRead),
		},
		Groups: []router.Group{{
			Middlewares: []router.Middleware{subnetWrite},
			Routes: []router.Route{
				router.Post("/{az}/{projectId}/subnet", s.CreateSubnet, quota),
				router.Post("/{az}/{projectId}/subnet/{effectiveId}", s.UpdateSubnet),
				router.Delete("/{az}/{projectId}/subnet/{effectiveId}", s.DeleteSubnet),
			},
		}},
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
