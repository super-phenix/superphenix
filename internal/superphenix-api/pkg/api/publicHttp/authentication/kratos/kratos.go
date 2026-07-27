package kratos

import (
	"context"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/provider"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	kratos "github.com/ory/kratos-client-go"
	"github.com/rs/zerolog/log"
)

var (
	kratosApiClient *kratos.APIClient
)

var (
	kratosCookie = config.Global.Authentication.KratosCookie
)

func init() {
	cfg := kratos.NewConfiguration()
	cfg.Servers = kratos.ServerConfigurations{{URL: config.Global.Authentication.KratosEndpoint}}
	kratosApiClient = kratos.NewAPIClient(cfg)
}

// Authenticator is a JWT middleware capable to authenticate HTTP requests
func Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(kratosCookie)
		if err != nil || cookie == nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		session, _, err := kratosApiClient.FrontendAPI.
			ToSession(r.Context()).
			Cookie(cookie.String()).
			Execute()

		if err != nil {
			log.Error().Ctx(r.Context()).Err(err).Msg("Error getting session")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if session.AuthenticatedAt == nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		if *session.Active == false {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		// Auto enroll user and add userId to context
		r = provider.InitializeKratosUser(r, session)

		r = r.WithContext(context.WithValue(r.Context(), consts.ContextSessionId, session.Identity.Id))

		next.ServeHTTP(w, r)
	})
}
