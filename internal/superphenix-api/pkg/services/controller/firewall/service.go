package firewall

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-firewall"

// API is the overridable seam for the firewall endpoints; the
// methods are the HTTP handlers.
type API interface {
	ListFirewalls(http.ResponseWriter, *http.Request)
	ListAZFirewalls(http.ResponseWriter, *http.Request)
	GetFirewall(http.ResponseWriter, *http.Request)
	CreateFirewall(http.ResponseWriter, *http.Request)
	UpdateFirewall(http.ResponseWriter, *http.Request)
	DeleteFirewall(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the firewall routes for any API.
func Module(cfg *config.Config, s API) router.Module {
	var (
		firewallRead  = controller.Perm(pwPermission.ProjectFirewallRead)
		firewallWrite = controller.Perm(pwPermission.ProjectFirewallWrite)
		quota         = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(moduleName, nil, router.Group{
		Middlewares: []router.Middleware{firewallRead},
		Routes: []router.Route{
			router.Get("/{projectId}/firewall", s.ListFirewalls),
			router.Get("/{az}/{projectId}/firewall", s.ListAZFirewalls),
			router.Get("/{az}/{projectId}/firewall/{effectiveId}", s.GetFirewall),
		},
		Groups: []router.Group{{
			Middlewares: []router.Middleware{firewallWrite},
			Routes: []router.Route{
				router.Post("/{az}/{projectId}/firewall", s.CreateFirewall, quota),
				router.Post("/{az}/{projectId}/firewall/{effectiveId}", s.UpdateFirewall),
				router.Delete("/{az}/{projectId}/firewall/{effectiveId}", s.DeleteFirewall),
			},
		}},
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
