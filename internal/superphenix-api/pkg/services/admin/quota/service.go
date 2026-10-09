package quota

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/adminHttp/authentication"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
)

const ModuleName = "admin_quota"

type API interface {
	GetOrganizationQuota(http.ResponseWriter, *http.Request)
	SetOrganizationQuota(http.ResponseWriter, *http.Request)
	DeleteOrganizationQuota(http.ResponseWriter, *http.Request)
}

type Service struct {
	cfg *config.Config
}

var _ API = (*Service)(nil)

func New(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

func Module(cfg *config.Config, s API) router.Module {
	return router.Module{
		Name:        ModuleName,
		Mount:       "/quota",
		Middlewares: []router.Middleware{authentication.SecretAuth(cfg.AdminHTTP.AuthSecret)},
		Routes: []router.Route{
			router.Get("/organization/{orgaId}", s.GetOrganizationQuota),
			router.Put("/organization/{orgaId}", s.SetOrganizationQuota),
			router.Delete("/organization/{orgaId}", s.DeleteOrganizationQuota),
		},
	}
}

func ProvideService(cfg *config.Config, reg *router.Registry) {
	h := New(cfg)
	reg.Register(Module(cfg, h))
}
