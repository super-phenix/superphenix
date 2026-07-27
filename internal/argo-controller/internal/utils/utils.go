package utils

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/super-phenix/superphenix/internal/argo-controller/pkg/config"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/go-chi/chi/v5"
)

const (
	UserContext = "UserId"
	UserHeader  = "X-User-Id"
)

func AddUserToContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userId := r.Header.Get(UserHeader)
		r = r.WithContext(context.WithValue(r.Context(), UserContext, userId))
		next.ServeHTTP(w, r)
	})
}

func GetRequestNamespace(r *http.Request) string {
	projectId := chi.URLParam(r, "projectId")
	return GetNamespace(projectId)
}

func GetNamespace(projectId string) string {
	return fmt.Sprintf(`%s-%s`, config.Global.SpxPrefix, projectId)
}

// CheckOrganizationWhitelist middleware check if the organization is allowed in this controller
func CheckOrganizationWhitelist() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := logger.GetLogger(r.Context())
			orgId := chi.URLParam(r, "orgId")

			// If a whitelist is defined and not empty
			if config.Global.OrganizationWhitelist != nil && len(config.Global.OrganizationWhitelist) > 0 {
				// If the organization is allowed
				if orgId != "" && slices.Contains(config.Global.OrganizationWhitelist, orgId) {
					next.ServeHTTP(w, r)
				} else {
					log.Error().Str("orgId", orgId).Msg("Organization is not whitelisted")
					httpError.Http(w, r, http.StatusForbidden).Msg(http.StatusText(http.StatusForbidden))
				}
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}
