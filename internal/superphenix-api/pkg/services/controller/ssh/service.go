package ssh

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-ssh"

// API is the overridable seam for the SSH endpoints; the methods are the
// HTTP handlers.
type API interface {
	ListSSHs(http.ResponseWriter, *http.Request)
	ListAZSSHs(http.ResponseWriter, *http.Request)
	GetSSH(http.ResponseWriter, *http.Request)
	CreateSSH(http.ResponseWriter, *http.Request)
	DeleteSSH(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the SSH routes for any API.
func Module(cfg *config.Config, s API) router.Module {
	var (
		sshRead  = controller.Perm(pwPermission.ProjectSSHRead)
		sshWrite = controller.Perm(pwPermission.ProjectSSHWrite)
		quota    = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(moduleName, nil, router.Group{
		Middlewares: []router.Middleware{sshRead},
		Routes: []router.Route{
			router.Get("/{projectId}/ssh", s.ListSSHs),
			router.Get("/{az}/{projectId}/ssh", s.ListAZSSHs),
			router.Get("/{az}/{projectId}/ssh/{effectiveId}", s.GetSSH),
		},
		Groups: []router.Group{{
			Middlewares: []router.Middleware{sshWrite},
			Routes: []router.Route{
				router.Post("/{az}/{projectId}/ssh", s.CreateSSH, quota),
				router.Delete("/{az}/{projectId}/ssh/{effectiveId}", s.DeleteSSH),
			},
		}},
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
