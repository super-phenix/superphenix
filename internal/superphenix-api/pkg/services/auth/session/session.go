package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authentication/jwt"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/user"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

var RefreshAuth = authentication.AuthType{
	Name:       "RefreshAuth",
	Detection:  detection,
	Validation: validate,
}

func detection(w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie(config.Global.Session.Cookies.Name)
	if err == nil && cookie != nil {
		return true
	}
	return false
}

func validate(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	log := logger.GetLogger(r.Context())
	cookie, err := r.Cookie(config.Global.Session.Cookies.Name)
	if err != nil || cookie == nil {
		log.Error().Err(err).Msg("Cookie not found")
		return r, err
	}

	claims, err := jwt.ParseTokenString(cookie.Value)
	if err != nil {
		log.Error().Err(err).Msg("Invalid token")
		return r, err
	}

	if !jwt.IsAudience(claims, jwt.RefreshAudience) {
		log.Error().Err(err).Msg("The token is not a refresh token")
		return r, fmt.Errorf("the token is not a refresh token")
	}

	// Token is authenticated, pass it through
	ctx := jwt.UpdateRequestContext(r.Context(), claims)
	r = r.WithContext(ctx)

	// Fetch matching user and add it to context if the user is valid
	r, err = authentication.RetrieveUserFromSession(r)
	if err != nil {
		log.Error().Err(err).Msg("Error retrieving user from session")
		return r, err
	}

	return r, nil
}

// RetrieveAccessToken creates a new JWT Access Token
//
//	@Summary		Generate an Access Token
//	@Description	Generates an access token based on refresh token
//	@Tags			v1, session
//	@Produce		plain
//	@Success		200	{string}	string	"JWT"
//	@Failure		500
//	@Router			/v1/session [get]
//	@Security		RefreshToken
func (h *Service) RetrieveAccessToken(w http.ResponseWriter, r *http.Request) {
	id := r.Context().Value(consts.ContextSessionId)
	log := logger.GetLogger(r.Context())
	if id == nil {
		log.Error().Msg("No id found in context")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	idStr := id.(string)

	userId := r.Context().Value(consts.ContextUserId)
	if userId == nil {
		log.Error().Msg("No user Id found in context")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// If user is not active in db, send contact admin error
	if ok, err := user.IsUserActive(userId.(string)); ok == false || err != nil {
		contactAdminResponse(w, r)
		return
	}

	accessToken, err := jwt.CreateAccessToken(h.cfg.Session.AccessValidity, idStr)
	if err != nil {
		log.Error().Err(err).Msg("Error generating access token")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	apiUser, err := user.FindWithOrganizationByProvider(idStr, db.KratosProvider)
	if err != nil {
		log.Error().Err(err).Msg("Failed to find user in database")
	}

	body := struct {
		Session string        `json:"session"`
		User    model.APIUser `json:"user"`
	}{
		Session: accessToken,
		User:    apiUser,
	}

	marshal, err := json.Marshal(body)
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling response")
	}
	_, _ = w.Write(marshal)
}

// GenerateTokens creates a JWT Access Token and a JWT Refresh Token
//
//	@Summary		Generate Access and Refresh Tokens
//	@Description	Generates an access token and an associated refresh token
//	@Tags			v1, session
//	@Produce		plain
//	@Success		302
//	@Failure		500
//	@Router			/v1/session/token [get]
//	@Security		Bearer
func (h *Service) GenerateTokens(writer http.ResponseWriter, request *http.Request) {
	sessionId := request.Context().Value(consts.ContextSessionId)
	log := logger.GetLogger(request.Context())
	if sessionId == nil {
		log.Error().Msg("No sessionId found in context")
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	sessionIdStr := sessionId.(string)

	userId := request.Context().Value(consts.ContextUserId)
	if userId == nil {
		log.Error().Msg("No user Id found in context")
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// If user is not active in db, send contact admin error
	if ok, err := user.IsUserActive(userId.(string)); ok == false || err != nil {
		log.Info().Str("user_id", userId.(string)).Msg("User is inactive")
		contactAdminResponse(writer, request)
		return
	}

	returnTo := request.URL.Query().Get("return_to")
	urlReturnTo := parseReturnUrl(request.Context(), returnTo)

	tokens, err := jwt.CreateAccessAndRefreshTokens(h.cfg.Session.AccessValidity, h.cfg.Session.RefreshValidity, sessionIdStr)
	if err != nil {
		log.Error().Err(err).Msg("Error generating access and refresh tokens")
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	http.SetCookie(writer, &http.Cookie{
		Name:     h.cfg.Session.Cookies.Name,
		Domain:   h.cfg.Session.Cookies.Domain,
		SameSite: getSameSite(h.cfg.Session.Cookies.SameSite),
		Path:     h.cfg.Session.Cookies.Path,
		Expires:  time.Now().Add(h.cfg.Session.RefreshValidity),
		MaxAge:   int(h.cfg.Session.RefreshValidity.Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.Session.Cookies.Secure,
		Value:    tokens[jwt.RefreshToken],
	})
	// The access token rides in the callback URL because the SPA needs a way
	// to pick it up from the redirect target. Callers should treat this URL
	// as sensitive: allowedOrigins gates which frontends may receive it.
	http.Redirect(writer, request, fmt.Sprintf("%ssession=%s", urlReturnTo, tokens[jwt.AccessToken]), http.StatusFound)
}

// Logout invalidate a refresh cookie and redirect to default url
//
//	@Summary		Logout Current User
//	@Description	Invalidate the refresh token and redirect to default url
//	@Tags			v1, session
//	@Produce		json
//	@Success		302
//	@Router			/v1/logout [get]
func (h *Service) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.Session.Cookies.Name,
		Domain:   h.cfg.Session.Cookies.Domain,
		SameSite: getSameSite(h.cfg.Session.Cookies.SameSite),
		Path:     h.cfg.Session.Cookies.Path,
		Expires:  time.Now(),
		MaxAge:   0,
		HttpOnly: true,
		Secure:   h.cfg.Session.Cookies.Secure,
		Value:    "",
	})
	http.Redirect(w, r, h.cfg.Session.DefaultReturnUrl, http.StatusFound)
}

// WhoAmI return the currently logged user
//
//	@Summary		Get Current User Information
//	@Description	Get current user information based on access token
//	@Tags			v1, session
//	@Produce		json
//	@Success		200	{object}	model.APIUser	"User"
//	@Failure		500
//	@Router			/v1/whoami [get]
//	@Security		Bearer
func (h *Service) WhoAmI(w http.ResponseWriter, r *http.Request) {
	userId := r.Context().Value(consts.ContextUserId)
	log := logger.GetLogger(r.Context())
	if userId == nil {
		log.Error().Msg("No id found in context")
		_, _ = w.Write(model.APIUserEmpty)
		return
	}

	userResult, err := user.FindWithOrganization(userId.(string))
	if err != nil {
		log.Error().Err(err).Msg("Error finding user")
		_, _ = w.Write(model.APIUserEmpty)
		return
	}

	userStr, err := json.Marshal(userResult)
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling user")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	_, _ = w.Write(userStr)
}

func getSameSite(sameSite string) http.SameSite {
	switch sameSite {
	case "Strict":
		return http.SameSiteStrictMode
	case "Lax":
		return http.SameSiteLaxMode
	case "None":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteDefaultMode
	}
}

// parseReturnUrl create a callback url with returnTo query param based a string url
func parseReturnUrl(ctx context.Context, returnTo string) string {
	log := logger.GetLogger(ctx)
	urlParse, err := url.Parse(returnTo)

	if err != nil || urlParse.Host == "" {
		if returnTo != "" {
			log.Warn().Str("return_to", returnTo).Msg("Return URL invalid or missing host, using default")
		}
		return config.Global.Session.DefaultReturnUrl + "?"
	}

	// Validate origin against AllowedOrigins. Wildcard "*" is only honoured
	// when the operator has explicitly set session.cors.allowUnsafeWildcard,
	// which config validation only permits on non-production deployments.
	allowed := false
	origin := fmt.Sprintf("%s://%s", urlParse.Scheme, urlParse.Host)
	for _, allowedOrigin := range config.Global.Session.Cors.AllowedOrigins {
		if allowedOrigin == "*" && config.Global.Session.Cors.AllowUnsafeWildcard {
			allowed = true
			break
		}
		if strings.EqualFold(allowedOrigin, origin) {
			allowed = true
			break
		}
	}

	if !allowed {
		log.Warn().Str("return_to", returnTo).Msg("Return URL not allowed, using default")
		return config.Global.Session.DefaultReturnUrl + "?"
	}

	// Create callback url from returnTo origin, and add the redirect Path
	// This will be used by Angular for automatic navigation
	returnPath := urlParse.Path
	if returnPath == "" {
		returnPath = "/"
	}
	return fmt.Sprintf("%s://%s/callback?return_to=%s&", urlParse.Scheme, urlParse.Host, returnPath)
}

func contactAdminResponse(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, fmt.Sprintf("%snot_active=%t", config.Global.Session.DefaultReturnUrl+"?", true), http.StatusFound)
	return
}
