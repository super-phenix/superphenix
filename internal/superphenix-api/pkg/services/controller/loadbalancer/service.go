package loadbalancer

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-load-balancer"

// API is the overridable seam for the load-balancer endpoints;
// the methods are the HTTP handlers.
type API interface {
	ListLoadBalancers(http.ResponseWriter, *http.Request)
	ListAZLoadBalancers(http.ResponseWriter, *http.Request)
	GetLoadBalancer(http.ResponseWriter, *http.Request)
	CreateLoadBalancer(http.ResponseWriter, *http.Request)
	UpdateLoadBalancer(http.ResponseWriter, *http.Request)
	DeleteLoadBalancer(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the load-balancer routes for any API.
func Module(cfg *config.Config, s API) router.Module {
	var (
		loadBalancerRead  = controller.Perm(pwPermission.ProjectLoadBalancerRead)
		loadBalancerWrite = controller.Perm(pwPermission.ProjectLoadBalancerWrite)
		quota             = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(moduleName, nil, router.Group{
		Middlewares: []router.Middleware{loadBalancerRead},
		Routes: []router.Route{
			router.Get("/{projectId}/load-balancer", s.ListLoadBalancers),
			router.Get("/{az}/{projectId}/load-balancer", s.ListAZLoadBalancers),
			router.Get("/{az}/{projectId}/load-balancer/{effectiveId}", s.GetLoadBalancer),
		},
		Groups: []router.Group{{
			Middlewares: []router.Middleware{loadBalancerWrite},
			Routes: []router.Route{
				router.Post("/{az}/{projectId}/load-balancer", s.CreateLoadBalancer, quota),
				router.Post("/{az}/{projectId}/load-balancer/{effectiveId}", s.UpdateLoadBalancer),
				router.Delete("/{az}/{projectId}/load-balancer/{effectiveId}", s.DeleteLoadBalancer),
			},
		}},
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
