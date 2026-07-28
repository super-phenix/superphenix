package vmsnapshot

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-vm-snapshot"

// API is the overridable seam for the VM-snapshot endpoints; the
// methods are the HTTP handlers.
type API interface {
	ListVmSnapshots(http.ResponseWriter, *http.Request)
	ListAZVmSnapshots(http.ResponseWriter, *http.Request)
	GetVmSnapshot(http.ResponseWriter, *http.Request)
	CreateVmSnapshot(http.ResponseWriter, *http.Request)
	DeleteVmSnapshot(http.ResponseWriter, *http.Request)
	RestoreVmSnapshot(http.ResponseWriter, *http.Request)
	CloneVmSnapshot(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the VM-snapshot routes for any API.
func Module(cfg *config.Config, s API) router.Module {
	var (
		snapshotRead  = controller.Perm(pwPermission.ProjectSnapshotRead)
		snapshotWrite = controller.Perm(pwPermission.ProjectSnapshotWrite)
		instanceWrite = controller.Perm(pwPermission.ProjectInstanceWrite)
		quota         = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(moduleName, nil, router.Group{
		Middlewares: []router.Middleware{snapshotRead},
		Routes: []router.Route{
			router.Get("/{projectId}/instance-snapshot", s.ListVmSnapshots),
			router.Get("/{az}/{projectId}/instance-snapshot", s.ListAZVmSnapshots),
			router.Get("/{az}/{projectId}/instance-snapshot/{effectiveId}", s.GetVmSnapshot),
		},
		Groups: []router.Group{{
			Middlewares: []router.Middleware{snapshotWrite},
			Routes: []router.Route{
				router.Post("/{az}/{projectId}/instance-snapshot", s.CreateVmSnapshot, quota),
				router.Delete("/{az}/{projectId}/instance-snapshot/{effectiveId}", s.DeleteVmSnapshot),
				router.Get("/{az}/{projectId}/instance-snapshot/{effectiveId}/restore", s.RestoreVmSnapshot, instanceWrite),
				router.Post("/{az}/{projectId}/instance-snapshot/{effectiveId}/clone", s.CloneVmSnapshot, instanceWrite),
			},
		}},
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
