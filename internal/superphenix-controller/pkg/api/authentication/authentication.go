package authentication

import (
	"net/http"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

// BearerAuth implements a simple middleware handler for adding bearer http auth to a route.
func BearerAuth() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if bearer, ok := bearerAuth(r); !ok || bearer != config.Global.Http.AuthSecret {
				log := logger.GetLogger(r.Context())
				log.Error().Str("bearer", bearer).Msg("Bearer auth failed")
				bearerAuthFailed(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func bearerAuthFailed(w http.ResponseWriter) {
	w.WriteHeader(http.StatusUnauthorized)
}

func bearerAuth(r *http.Request) (bearer string, ok bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", false
	}
	return parseBearerAuth(auth)
}

func parseBearerAuth(auth string) (bearer string, ok bool) {
	const prefix = "Bearer "
	if len(auth) < len(prefix) {
		return "", false
	}
	bearer, ok = strings.CutPrefix(auth, prefix)
	if !ok {
		return "", false
	}
	return bearer, true
}
