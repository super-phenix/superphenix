package controller

import (
	"net/http"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authorization/permify"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/product"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/project"
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

// NewControllerModule builds a controller route module with the shared mount,
// middleware chain and enable gate. Each per-resource module supplies its own
// Name, flat Routes and/or permission-scoped Groups; the rest is identical
// across resources. routes carries ungrouped routes (whose permission chain
// doesn't fit a shared scope); groups carries permission-scoped route trees
// built with Scope.
func NewControllerModule(name string, routes []router.Route, groups ...router.Group) router.Module {
	return router.Module{
		Name:        name,
		Mount:       "/{orgaId}" + config.ApiPrefix,
		Middlewares: SharedMiddlewares(),
		Enabled:     func() bool { return true },
		Routes:      routes,
		Groups:      groups,
	}
}

// SharedMiddlewares is the chain applied to every controller route: auth, user-id
// propagation, the baseline organization-read check and the refusal of deactivated projects.
func SharedMiddlewares() []router.Middleware {
	return []router.Middleware{
		authentication.Authenticate(jwt.JwtBearerAuth, apiToken.ApiTokenAuth),
		proxy.AddUserIdToRequestHeader,
		Perm(pwPermission.OrganizationRead),
		CheckProjectActive,
	}
}

func Perm(permission string) router.Middleware { return permify.CheckPermission(permission) }

// CheckProjectActive rejects with 423 Locked any request on a deactivated project (see
// model.ProjectDeactivation). Routes without a {projectId} are not concerned.
func CheckProjectActive(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgaUuid, errOrga := uuid.Parse(chi.URLParam(r, "orgaId"))
		projectUuid, errProject := uuid.Parse(chi.URLParam(r, "projectId"))
		if errOrga != nil || errProject != nil {
			// No (valid) project: left to the handler.
			next.ServeHTTP(w, r)
			return
		}

		deactivations, err := project.ListDeactivations(r.Context(), orgaUuid, projectUuid)
		if err != nil {
			log.Error().Err(err).Str("projectId", projectUuid.String()).Msg("Failed to check project deactivation")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if len(deactivations) > 0 {
			reasons := make([]string, len(deactivations))
			for i, d := range deactivations {
				reasons[i] = d.Reason
			}
			http.Error(w, "project deactivated: "+strings.Join(reasons, ", "), http.StatusLocked)
			return
		}

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
