package disk

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-disk"

// API is the overridable seam for the disk endpoints; the methods are
// the HTTP handlers (named handlers and SimpleRedirect passthroughs).
type API interface {
	ListDisks(http.ResponseWriter, *http.Request)
	ListAZDisks(http.ResponseWriter, *http.Request)
	GetDisk(http.ResponseWriter, *http.Request)
	CreateDisk(http.ResponseWriter, *http.Request)
	UpdateDisk(http.ResponseWriter, *http.Request)
	DeleteDisk(http.ResponseWriter, *http.Request)
	Unmount(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the disk routes for any API.
func Module(cfg *config.Config, s API) router.Module {
	var (
		diskRead      = controller.Perm(pwPermission.ProjectDiskRead)
		diskWrite     = controller.Perm(pwPermission.ProjectDiskWrite)
		instanceRead  = controller.Perm(pwPermission.ProjectInstanceRead)
		instanceWrite = controller.Perm(pwPermission.ProjectInstanceWrite)
		quota         = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(moduleName,
		[]router.Route{
			router.Get("/{az}/{projectId}/disk/{effectiveId}/unmount", s.Unmount, instanceRead, diskRead, instanceWrite).
				Audited(model.ProductTypeDisk, controller.ActionUnmount, controller.ParamEffectiveID),
		},
		router.Group{
			Middlewares: []router.Middleware{diskRead},
			Routes: []router.Route{
				router.Get("/{projectId}/disk", s.ListDisks),
				router.Get("/{az}/{projectId}/disk", s.ListAZDisks),
				router.Get("/{az}/{projectId}/disk/{effectiveId}", s.GetDisk),
			},
			Groups: []router.Group{{
				Middlewares: []router.Middleware{diskWrite},
				Routes: []router.Route{
					router.Post("/{az}/{projectId}/disk", s.CreateDisk, quota).
						Audited(model.ProductTypeDisk, router.ActionCreate, ""),
					router.Post("/{az}/{projectId}/disk/{effectiveId}", s.UpdateDisk).
						Audited(model.ProductTypeDisk, router.ActionUpdate, controller.ParamEffectiveID),
					router.Delete("/{az}/{projectId}/disk/{effectiveId}", s.DeleteDisk).
						Audited(model.ProductTypeDisk, router.ActionDelete, controller.ParamEffectiveID),
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
