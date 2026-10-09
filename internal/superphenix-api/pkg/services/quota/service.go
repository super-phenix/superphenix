package quota

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authorization/permify"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication/jwt"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	apiToken "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/auth/apitoken"
	pwPerm "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
)

const ModuleName = "quota"

var auditResource = router.Resource{Name: "quota", Label: "Quota"}

type API interface {
	GetOrganizationQuota(http.ResponseWriter, *http.Request)
	GetProjectQuota(http.ResponseWriter, *http.Request)
	SetProjectQuota(http.ResponseWriter, *http.Request)
	SetProjectAZQuota(http.ResponseWriter, *http.Request)
	DeleteProjectQuota(http.ResponseWriter, *http.Request)
}

type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

func New(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

func Module(s API) router.Module {
	var (
		jwtOrToken   = authentication.Authenticate(jwt.JwtBearerAuth, apiToken.ApiTokenAuth)
		orgaRead     = permify.CheckPermission(pwPerm.OrganizationRead)
		orgaProjMgmt = permify.CheckPermission(pwPerm.OrganizationProjectManagement)
	)

	return router.Module{
		Name:  ModuleName,
		Mount: "/v1",
		Routes: []router.Route{
			router.Get("/organization/{orgaId}/quota", s.GetOrganizationQuota, jwtOrToken, orgaRead),
			router.Get("/organization/{orgaId}/project/{projectId}/quota", s.GetProjectQuota, jwtOrToken, orgaRead),
			router.Put("/organization/{orgaId}/project/{projectId}/quota", s.SetProjectQuota, jwtOrToken, orgaRead, orgaProjMgmt).
				Audited(auditResource, router.ActionUpdate, "projectId"),
			router.Put("/organization/{orgaId}/project/{projectId}/quota/az/{codeAZ}", s.SetProjectAZQuota, jwtOrToken, orgaRead, orgaProjMgmt).
				Audited(auditResource, router.ActionUpdate, "projectId"),
			router.Delete("/organization/{orgaId}/project/{projectId}/quota", s.DeleteProjectQuota, jwtOrToken, orgaRead, orgaProjMgmt).
				Audited(auditResource, router.ActionDelete, "projectId"),
		},
	}
}

func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(h))
}
