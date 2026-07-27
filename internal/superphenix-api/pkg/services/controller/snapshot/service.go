package snapshot

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-snapshot"

// API is the overridable seam for the snapshot endpoints; the methods are the
// HTTP handlers.
type API interface {
	ListSnapshots(http.ResponseWriter, *http.Request)
	ListAZSnapshots(http.ResponseWriter, *http.Request)
	GetSnapshot(http.ResponseWriter, *http.Request)
	CreateSnapshot(http.ResponseWriter, *http.Request)
	UpdateSnapshot(http.ResponseWriter, *http.Request)
	DeleteSnapshot(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the snapshot routes for any API.
func Module(cfg *config.Config, s API) router.Module {
	var (
		snapshotRead  = controller.Perm(pwPermission.ProjectSnapshotRead)
		snapshotWrite = controller.Perm(pwPermission.ProjectSnapshotWrite)
		quota         = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(cfg, moduleName, nil, router.Group{
		Middlewares: []router.Middleware{snapshotRead},
		Routes: []router.Route{
			router.Get("/{projectId}/snapshot", s.ListSnapshots),
			router.Get("/{az}/{projectId}/snapshot", s.ListAZSnapshots),
			router.Get("/{az}/{projectId}/snapshot/{effectiveId}", s.GetSnapshot),
		},
		Groups: []router.Group{{
			Middlewares: []router.Middleware{snapshotWrite},
			Routes: []router.Route{
				router.Post("/{az}/{projectId}/snapshot", s.CreateSnapshot, quota),
				router.Post("/{az}/{projectId}/snapshot/{effectiveId}", s.UpdateSnapshot),
				router.Delete("/{az}/{projectId}/snapshot/{effectiveId}", s.DeleteSnapshot),
			},
		}},
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
