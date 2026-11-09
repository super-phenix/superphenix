package kratos

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/provider"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/audit"
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

// authTypeName is the auth type stored with the audit events of cookie sessions.
const authTypeName = "KratosSession"

// secondFactorRedirect returns the Kratos login URL when err asks for a second factor, or "" otherwise.
// Kratos never adds return_to to this URL and validates it against its own allowed_return_urls.
func secondFactorRedirect(err error, returnTo string) string {
	var apiErr *kratos.GenericOpenAPIError
	if !errors.As(err, &apiErr) {
		return ""
	}
	var body kratos.ErrorAuthenticatorAssuranceLevelNotSatisfied
	if json.Unmarshal(apiErr.Body(), &body) != nil || body.Error.GetId() != "session_aal2_required" || body.RedirectBrowserTo == nil {
		return ""
	}
	target, parseErr := url.Parse(*body.RedirectBrowserTo)
	if parseErr != nil {
		return ""
	}
	// The console sends the literal "null" when it has no return_to.
	if parsed, parseErr := url.Parse(returnTo); parseErr == nil && parsed.Host != "" {
		query := target.Query()
		query.Set("return_to", returnTo)
		target.RawQuery = query.Encode()
	}
	return target.String()
}

// Authenticator is a JWT middleware capable to authenticate HTTP requests
func Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(kratosCookie)
		if err != nil || cookie == nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		session, resp, err := kratosApiClient.FrontendAPI.
			ToSession(r.Context()).
			Cookie(cookie.String()).
			Execute()

		if err != nil {
			if target := secondFactorRedirect(err, r.URL.Query().Get("return_to")); target != "" {
				http.Redirect(w, r, target, http.StatusFound)
				return
			}
			if resp != nil && resp.StatusCode == http.StatusUnauthorized {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
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

		if userId, ok := r.Context().Value(consts.ContextUserId).(string); ok {
			audit.Begin(r.Context(), userId, authTypeName)
		}

		next.ServeHTTP(w, r)
	})
}
