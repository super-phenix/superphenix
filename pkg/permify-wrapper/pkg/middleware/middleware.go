package middleware

import (
	"net/http"
	p "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/permission"

	"github.com/rs/zerolog/log"
)

const (
	TenantContext  = "tenantId"
	EntityContext  = "entityId"
	SubjectContext = "subjectId"
	NoValue        = "-1"
)

func CanAccess(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantId := r.Context().Value(TenantContext)
			if tenantId == nil {
				log.Error().Msg("No tenant id found in context")
				tenantId = ""
			}
			entityId := r.Context().Value(EntityContext)
			if entityId == nil {
				log.Error().Msg("No entity id found in context")
				entityId = ""
			}
			subjectId := r.Context().Value(SubjectContext)
			if subjectId == nil {
				log.Error().Msg("No subject id found in context")
				subjectId = ""
			}

			canAccess := p.CanAccess(r.Context(), tenantId.(string), permission, entityId.(string), subjectId.(string))

			if canAccess {
				next.ServeHTTP(w, r)
			} else {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			}
		})
	}
}
