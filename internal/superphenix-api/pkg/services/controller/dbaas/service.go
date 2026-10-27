package dbaas

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/app"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"
	argoApp "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller/argo-app"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-dbaas"

// API is the overridable seam for the DBaaS endpoints; the methods are the HTTP
// handlers.
type API interface {
	ListDBaaS(http.ResponseWriter, *http.Request)
	GetPostgresVersions(http.ResponseWriter, *http.Request)
	GetDBaaS(http.ResponseWriter, *http.Request)
	GetForUpdateDBaaS(http.ResponseWriter, *http.Request)
	CreateDBaaS(http.ResponseWriter, *http.Request)
	UpdateDBaaS(http.ResponseWriter, *http.Request)
	DeleteDBaaS(http.ResponseWriter, *http.Request)
	RestartDBaaS(http.ResponseWriter, *http.Request)
	GetDBaaSCredentials(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg  *config.Config
	argo argoApp.Client
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects; the Argo client is
// injected so tests can fake it.
func New(cfg *config.Config, argoClient argoApp.Client) *Service {
	return &Service{cfg: cfg, argo: argoClient}
}

// Module builds the DBaaS routes for any API.
func Module(cfg *config.Config, s API) router.Module {
	var (
		dbaasRead  = controller.Perm(pwPermission.ProjectDBaaSRead)
		dbaasWrite = controller.Perm(pwPermission.ProjectDBaaSWrite)
		quota      = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(moduleName, nil, router.Group{
		Middlewares: []router.Middleware{dbaasRead},
		Routes: []router.Route{
			router.Get("/{projectId}/dbaas", s.ListDBaaS),
			router.Get("/{projectId}/dbaas/postgres-versions", s.GetPostgresVersions),
			router.Get("/{az}/{projectId}/dbaas/{effectiveId}", s.GetDBaaS),
			router.Get("/{az}/{projectId}/dbaas/{effectiveId}/app", s.GetForUpdateDBaaS),
		},
		Groups: []router.Group{{
			Middlewares: []router.Middleware{dbaasWrite},
			Routes: []router.Route{
				router.Post("/{az}/{projectId}/dbaas", s.CreateDBaaS, quota).
					Audited(model.ProductTypeDBaaS, router.ActionCreate, ""),
				router.Post("/{az}/{projectId}/dbaas/{effectiveId}", s.UpdateDBaaS).
					Audited(model.ProductTypeDBaaS, router.ActionUpdate, controller.ParamEffectiveID),
				router.Delete("/{az}/{projectId}/dbaas/{effectiveId}", s.DeleteDBaaS).
					Audited(model.ProductTypeDBaaS, router.ActionDelete, controller.ParamEffectiveID),
				router.Post("/{az}/{projectId}/dbaas/{effectiveId}/restart", s.RestartDBaaS).
					Audited(model.ProductTypeDBaaS, controller.ActionRestart, controller.ParamEffectiveID),
				// The credentials hold the password, so they need the write permission.
				router.Get("/{az}/{projectId}/dbaas/{effectiveId}/credentials", s.GetDBaaSCredentials),
			},
		}},
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg, app.ProvideArgo(cfg))
	reg.Register(Module(cfg, h))
}
