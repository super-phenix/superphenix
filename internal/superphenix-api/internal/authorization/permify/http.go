package permify

import (
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authorization"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// CheckPermission middleware check the permission
//
// See permission sets in permify-wrapper pkg
// **Note**: where pw is permission package of permify-wrapper
func CheckPermission(permission string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			orgaId := chi.URLParam(r, "orgaId")
			projectId := chi.URLParam(r, "projectId")
			userId := r.Context().Value(consts.ContextUserId)

			// If permission is for organization, the projectId will not be used
			if authorization.CheckPermissionForRequest(r, permission) {
				next.ServeHTTP(w, r)
			} else {
				log.Debug().Str("orgaId", orgaId).Msgf("check %s for entity %s and user %s failed", permission, projectId, userId.(string))
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			}
		})
	}
}
