package instance

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-instance"

// API is the overridable seam for the instance endpoints; the
// methods are the HTTP handlers (named handlers and SimpleRedirect passthroughs).
type API interface {
	ListInstances(http.ResponseWriter, *http.Request)
	ListAZInstances(http.ResponseWriter, *http.Request)
	GetInstance(http.ResponseWriter, *http.Request)
	AdvancedOptions(http.ResponseWriter, *http.Request)
	CreateInstance(http.ResponseWriter, *http.Request)
	UpdateInstance(http.ResponseWriter, *http.Request)
	DeleteInstance(http.ResponseWriter, *http.Request)
	Serial(http.ResponseWriter, *http.Request)
	Vnc(http.ResponseWriter, *http.Request)
	Start(http.ResponseWriter, *http.Request)
	Stop(http.ResponseWriter, *http.Request)
	StopForce(http.ResponseWriter, *http.Request)
	Restart(http.ResponseWriter, *http.Request)
	ContainerDiskMount(http.ResponseWriter, *http.Request)
	ContainerDiskUnmount(http.ResponseWriter, *http.Request)
	ContainerDisks(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the instance routes for any API, preserving
// the per-route method, path and permission chain of the original controller.
func Module(cfg *config.Config, s API) router.Module {
	var (
		instanceRead     = controller.Perm(pwPermission.ProjectInstanceRead)
		instanceWrite    = controller.Perm(pwPermission.ProjectInstanceWrite)
		instanceControl  = controller.Perm(pwPermission.ProjectInstanceControl)
		instanceTerminal = controller.Perm(pwPermission.ProjectInstanceTerminal)
		diskRead         = controller.Perm(pwPermission.ProjectDiskRead)
		diskWrite        = controller.Perm(pwPermission.ProjectDiskWrite)
		quota            = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(cfg, moduleName,
		[]router.Route{
			router.Get("/{az}/{projectId}/container-disks", s.ContainerDisks, diskRead),
		},
		router.Group{
			Middlewares: []router.Middleware{instanceRead},
			Routes: []router.Route{
				router.Get("/{projectId}/instance", s.ListInstances),
				router.Get("/{az}/{projectId}/instance", s.ListAZInstances),
				router.Get("/{az}/{projectId}/instance/{effectiveId}", s.GetInstance),
				router.Get("/{az}/{projectId}/instance/{effectiveId}/advanced-options", s.AdvancedOptions),
				router.Get("/{az}/{projectId}/instance/{effectiveId}/serial", s.Serial, instanceTerminal),
				router.Get("/{az}/{projectId}/instance/{effectiveId}/vnc", s.Vnc, instanceTerminal),
				router.Get("/{az}/{projectId}/instance/{effectiveId}/start", s.Start, instanceControl),
				router.Get("/{az}/{projectId}/instance/{effectiveId}/stop", s.Stop, instanceControl),
				router.Get("/{az}/{projectId}/instance/{effectiveId}/stop-force", s.StopForce, instanceControl),
				router.Get("/{az}/{projectId}/instance/{effectiveId}/restart", s.Restart, instanceControl),
			},
			Groups: []router.Group{{
				Middlewares: []router.Middleware{instanceWrite},
				Routes: []router.Route{
					router.Post("/{az}/{projectId}/instance", s.CreateInstance, quota),
					router.Post("/{az}/{projectId}/instance/{effectiveId}", s.UpdateInstance),
					router.Delete("/{az}/{projectId}/instance/{effectiveId}", s.DeleteInstance),
					router.Post("/{az}/{projectId}/instance/{effectiveId}/container-disk/mount", s.ContainerDiskMount, diskRead, diskWrite),
					router.Post("/{az}/{projectId}/instance/{effectiveId}/container-disk/unmount", s.ContainerDiskUnmount, diskRead, diskWrite),
				},
			}},
		},
	)
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
