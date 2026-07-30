package kaas

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-kaas"

// API is the overridable seam for the KaaS endpoints; the methods are the HTTP
// handlers (named handlers and SimpleRedirect passthroughs).
type API interface {
	ListKaaS(http.ResponseWriter, *http.Request)
	GetKubeVersion(http.ResponseWriter, *http.Request)
	GetKaaS(http.ResponseWriter, *http.Request)
	CreateKaaS(http.ResponseWriter, *http.Request)
	UpdateKaaS(http.ResponseWriter, *http.Request)
	DeleteKaaS(http.ResponseWriter, *http.Request)
	GetForUpdateKaaS(http.ResponseWriter, *http.Request)
	ReinstallKaaSEssentials(http.ResponseWriter, *http.Request)
	Instances(http.ResponseWriter, *http.Request)
	Netpols(http.ResponseWriter, *http.Request)
	GetKaaSKubeConfig(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the KaaS routes for any API, preserving the
// per-route method, path and permission chain of the original controller.
func Module(cfg *config.Config, s API) router.Module {
	var (
		kaasRead       = controller.Perm(pwPermission.ProjectKaaSRead)
		kaasWrite      = controller.Perm(pwPermission.ProjectKaaSWrite)
		kaasKubeConfig = controller.Perm(pwPermission.ProjectKaaSKubeConfig)
		instanceRead   = controller.Perm(pwPermission.ProjectInstanceRead)
		firewallRead   = controller.Perm(pwPermission.ProjectFirewallRead)
		quota          = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(cfg, moduleName, nil, router.Group{
		Middlewares: []router.Middleware{kaasRead},
		Routes: []router.Route{
			router.Get("/{projectId}/kaas", s.ListKaaS),
			router.Get("/{projectId}/kaas/kube-versions", s.GetKubeVersion),
			router.Get("/{az}/{projectId}/kaas/{effectiveId}", s.GetKaaS),
			router.Get("/{az}/{projectId}/kaas/{effectiveId}/instances", s.Instances, instanceRead),
			router.Get("/{az}/{projectId}/kaas/{effectiveId}/netpols", s.Netpols, firewallRead),
			router.Get("/{az}/{projectId}/kaas/{effectiveId}/kubeconfig", s.GetKaaSKubeConfig, kaasKubeConfig),
		},
		Groups: []router.Group{{
			Middlewares: []router.Middleware{kaasWrite},
			Routes: []router.Route{
				router.Post("/{az}/{projectId}/kaas", s.CreateKaaS, quota),
				router.Post("/{az}/{projectId}/kaas/{effectiveId}", s.UpdateKaaS),
				router.Delete("/{az}/{projectId}/kaas/{effectiveId}", s.DeleteKaaS),
				router.Get("/{az}/{projectId}/kaas/{effectiveId}/app", s.GetForUpdateKaaS),
				router.Get("/{az}/{projectId}/kaas/{effectiveId}/reinstall-essentials", s.ReinstallKaaSEssentials),
			},
		}},
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
