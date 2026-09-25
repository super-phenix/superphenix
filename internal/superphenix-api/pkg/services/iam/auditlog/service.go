// Package auditlog exposes the audit events and their retention.
package auditlog

import (
	"context"
	"net/http"
	"sync"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authorization/permify"
	auditEvent "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/audit-event"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/organization"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication/jwt"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	apiToken "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/auth/apitoken"

	pwPerm "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"

	"github.com/google/uuid"
)

// ModuleName is the registry key for the audit log routes.
const ModuleName = "audit-log"

// API is the overridable seam for the audit log endpoints; the methods are the HTTP handlers.
type API interface {
	ListOrganizationEvents(http.ResponseWriter, *http.Request)
	ListUserEvents(http.ResponseWriter, *http.Request)
	ListOrganizationEventTypes(http.ResponseWriter, *http.Request)
	ListUserEventTypes(http.ResponseWriter, *http.Request)
	GetRetention(http.ResponseWriter, *http.Request)
	UpdateRetention(http.ResponseWriter, *http.Request)
	GetUserRetention(http.ResponseWriter, *http.Request)
}

// Store is what the service reads and writes.
type Store interface {
	List(ctx context.Context, filter auditEvent.Filter) ([]model.AuditEvent, int64, error)
	Retention(orgaId uuid.UUID) (*int, error)
	SetRetention(orgaId uuid.UUID, days *int) error
}

type dbStore struct{}

func (dbStore) List(ctx context.Context, filter auditEvent.Filter) ([]model.AuditEvent, int64, error) {
	return auditEvent.List(ctx, filter)
}

func (dbStore) Retention(orgaId uuid.UUID) (*int, error) {
	orga, err := organization.FindById(orgaId.String())
	return orga.AuditRetentionDays, err
}

func (dbStore) SetRetention(orgaId uuid.UUID, days *int) error {
	return organization.SetAuditRetentionDays(orgaId, days)
}

// Service is the default implementation of API.
type Service struct {
	cfg   *config.Config
	store Store

	// declared is read once at first use, after every module has registered. Nil means an empty
	// catalogue.
	declared     func() []router.RouteInfo
	once         sync.Once
	organization *catalogue
	user         *catalogue
}

var _ API = (*Service)(nil)

// New constructs the default service. It has no side effects.
func New(cfg *config.Config) *Service { return NewWithStore(cfg, dbStore{}) }

// NewWithStore constructs the service over another Store.
func NewWithStore(cfg *config.Config, store Store) *Service {
	return NewWithDeclared(cfg, store, nil)
}

// NewWithDeclared constructs the service over a Store and the routes the catalogue is built from.
func NewWithDeclared(cfg *config.Config, store Store, declared func() []router.RouteInfo) *Service {
	return &Service{cfg: cfg, store: store, declared: declared}
}

// catalogue returns the organization or the user catalogue, building both on first use.
func (s *Service) catalogue(forOrganization bool) *catalogue {
	s.once.Do(func() {
		var declared []router.RouteInfo
		if s.declared != nil {
			declared = s.declared()
		}
		s.organization, s.user = buildCatalogues(declared)
	})
	if forOrganization {
		return s.organization
	}
	return s.user
}

// Module builds the route module for any API implementation.
func Module(s API) router.Module {
	var (
		jwtOrToken = authentication.Authenticate(jwt.JwtBearerAuth, apiToken.ApiTokenAuth)
		orgaRead   = permify.CheckPermission(pwPerm.OrganizationRead)
		auditRead  = permify.CheckPermission(pwPerm.OrganizationAuditLogRead)
		auditWrite = permify.CheckPermission(pwPerm.OrganizationAuditLogWrite)
	)

	return router.Module{
		Name:  ModuleName,
		Mount: "/v1",
		Routes: []router.Route{
			router.Get("/organization/{orgaId}/audit-log", s.ListOrganizationEvents, jwtOrToken, orgaRead, auditRead),
			router.Get("/organization/{orgaId}/audit-log/event-types", s.ListOrganizationEventTypes, jwtOrToken, orgaRead, auditRead),
			router.Get("/organization/{orgaId}/audit-log/retention", s.GetRetention, jwtOrToken, orgaRead, auditRead),
			router.Post("/organization/{orgaId}/audit-log/retention", s.UpdateRetention, jwtOrToken, orgaRead, auditWrite).
				Audited(router.Resource{Name: "audit-log.retention", Label: "Audit log retention"}, router.ActionUpdate, "orgaId"),
			router.Get("/user/audit-log", s.ListUserEvents, jwtOrToken),
			router.Get("/user/audit-log/event-types", s.ListUserEventTypes, jwtOrToken),
			router.Get("/user/audit-log/retention", s.GetUserRetention, jwtOrToken),
		},
	}
}

// ProvideService constructs the default service and registers its routes on reg.
func ProvideService(cfg *config.Config, reg *router.Registry) {
	reg.Register(Module(NewWithDeclared(cfg, dbStore{}, reg.Declared)))
}
