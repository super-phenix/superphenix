package authentication

import (
	"net/http"

	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

// Authenticate is a middleware that detect and validate multiple auth method
func Authenticate(authTypes ...AuthType) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := logger.GetLogger(r.Context())

			// Auth Detection
			for _, authType := range authTypes {
				log.Info().Msgf("Checking %s authentication", authType.Name)
				// If the auth type match
				if authType.Detection(w, r) {
					// Make a validation of the authentication
					r, err := authType.Validation(w, r)
					if err != nil {
						log.Error().Err(err).Msgf("Validation failed for %s", authType.Name)
						http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
						return
					}
					next.ServeHTTP(w, r)
					return
				}
			}

			log.Error().Msg("No auth found")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		})
	}
}
