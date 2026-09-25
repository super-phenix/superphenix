package securitygroup

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/controller"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const moduleName = "spx-controller-security-group"

// API is the overridable seam for the security group endpoints; the
// methods are the HTTP handlers.
type API interface {
	ListSecurityGroups(http.ResponseWriter, *http.Request)
	ListAZSecurityGroups(http.ResponseWriter, *http.Request)
	GetSecurityGroup(http.ResponseWriter, *http.Request)
	CreateSecurityGroup(http.ResponseWriter, *http.Request)
	UpdateSecurityGroup(http.ResponseWriter, *http.Request)
	DeleteSecurityGroup(http.ResponseWriter, *http.Request)
}

// Service is the default implementation of API.
type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return &Service{cfg: cfg} }

// Module builds the security group routes for any API.
func Module(cfg *config.Config, s API) router.Module {
	var (
		securityGroupRead  = controller.Perm(pwPermission.ProjectSecurityGroupRead)
		securityGroupWrite = controller.Perm(pwPermission.ProjectSecurityGroupWrite)
		quota              = controller.CheckCreationQuota
	)
	return controller.NewControllerModule(moduleName, nil, router.Group{
		Middlewares: []router.Middleware{securityGroupRead},
		Routes: []router.Route{
			router.Get("/{projectId}/security-group", s.ListSecurityGroups),
			router.Get("/{az}/{projectId}/security-group", s.ListAZSecurityGroups),
			router.Get("/{az}/{projectId}/security-group/{effectiveId}", s.GetSecurityGroup),
		},
		Groups: []router.Group{{
			Middlewares: []router.Middleware{securityGroupWrite},
			Routes: []router.Route{
				router.Post("/{az}/{projectId}/security-group", s.CreateSecurityGroup, quota).
					Audited(model.ProductTypeSecurityGroup, router.ActionCreate, ""),
				router.Post("/{az}/{projectId}/security-group/{effectiveId}", s.UpdateSecurityGroup).
					Audited(model.ProductTypeSecurityGroup, router.ActionUpdate, controller.ParamEffectiveID),
				router.Delete("/{az}/{projectId}/security-group/{effectiveId}", s.DeleteSecurityGroup).
					Audited(model.ProductTypeSecurityGroup, router.ActionDelete, controller.ParamEffectiveID),
			},
		}},
	})
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
