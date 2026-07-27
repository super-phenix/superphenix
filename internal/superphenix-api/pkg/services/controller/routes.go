package controller

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authorization/permify"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/product"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication/jwt"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/proxy"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"
	apiToken "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/auth/apitoken"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// missingPrefixLog collapses the per-resource module builders into a single
// error-level log when the controller prefix is missing.
var missingPrefixLog sync.Once

// NewControllerModule builds a controller route module with the shared mount,
// middleware chain and enable gate. Each per-resource module supplies its own
// Name, flat Routes and/or permission-scoped Groups; the rest is identical
// across resources. routes carries ungrouped routes (whose permission chain
// doesn't fit a shared scope); groups carries permission-scoped route trees
// built with Scope.
func NewControllerModule(cfg *config.Config, name string, routes []router.Route, groups ...router.Group) router.Module {
	if cfg.Controller.ApiPrefix == "" {
		missingPrefixLog.Do(func() {
			log.Error().Msg("K8S Controller Prefix not found, couldn't setup Superphenix Controller Router")
		})
	}
	return router.Module{
		Name:        name,
		Mount:       "/{orgaId}" + cfg.Controller.ApiPrefix,
		Middlewares: SharedMiddlewares(),
		Enabled:     func() bool { return cfg.Controller.ApiPrefix != "" },
		Routes:      routes,
		Groups:      groups,
	}
}

// SharedMiddlewares is the chain applied to every controller route: auth, user-id
// propagation, bearer injection, and the baseline organization-read check.
func SharedMiddlewares() []router.Middleware {
	return []router.Middleware{
		authentication.Authenticate(jwt.JwtBearerAuth, apiToken.ApiTokenAuth),
		proxy.AddUserIdToRequestHeader,
		AddBearerHeader,
		Perm(pwPermission.OrganizationRead),
	}
}

func Perm(permission string) router.Middleware { return permify.CheckPermission(permission) }

// AddBearerHeader adds the controller's bearer secret to the proxied request.
func AddBearerHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set(consts.AuthorizationHeader, fmt.Sprintf("Bearer %s", config.Global.Controller.AuthSecret))
		next.ServeHTTP(w, r)
	})
}

// CheckCreationQuota rejects the request if the project's product creation quota is reached.
func CheckCreationQuota(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		projectId := chi.URLParam(r, "projectId")
		projectUuid, err := uuid.Parse(projectId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse uuid")
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		reached, err := product.IsQuotaCreationReached(r.Context(), projectUuid)
		if err != nil {
			log.Error().Err(err).Msg("Failed to check quota")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if reached {
			log.Error().Msg("Quota to create product reached")
			http.Error(w, "Quota to create product reached", http.StatusBadRequest)
			return
		}

		next.ServeHTTP(w, r)
	})
}
