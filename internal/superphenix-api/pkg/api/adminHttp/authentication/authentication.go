package authentication

import (
	"net/http"
)

// SecretAuth implements a simple middleware handler for adding bearer http auth to a route.
func SecretAuth(authSecret string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secret, ok := secretAuth(r); !ok || secret != authSecret {
				secretAuthFailed(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func secretAuthFailed(w http.ResponseWriter) {
	w.WriteHeader(http.StatusUnauthorized)
}

func secretAuth(r *http.Request) (secret string, ok bool) {
	auth := r.Header.Get("Secret")
	if auth == "" {
		return "", false
	}
	return auth, true
}
