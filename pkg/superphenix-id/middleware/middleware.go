package middleware

import (
	"context"
	"net/http"
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// AddEffectiveIdToContext add SPX prefixed effective id ({prefix}-{id}) into the request context
//
// This middleware require 3 path params
//   - orgId
//   - projectId
//   - localId
func AddEffectiveIdToContext() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			orgId := chi.URLParam(r, "orgId")
			projectId := chi.URLParam(r, "projectId")
			localId := chi.URLParam(r, "localId")

			var m spxId.Metadata
			// Check the information we've received respects the RFCs
			if err := m.GenerateMetadata(projectId, orgId, localId); err != nil {
				log.Err(err).Ctx(r.Context()).Msgf("error generating metadata")
			}

			r = r.WithContext(context.WithValue(r.Context(), spxId.EffectiveIdContext(), m.GetResourceEffectiveID()))

			next.ServeHTTP(w, r)
		})
	}
}
