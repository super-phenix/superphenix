package jwt

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authentication/jwt"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

var (
	illFormedTokenHeader = errors.New("authorization header format must be Bearer {token}")
	notFoundToken        = errors.New("token not found")
)

// JwtBearerAuth accepts a Superphenix API JWT in the Authorization header or
// in the "bearer" URL query parameter. URL transport is supported because
// some clients (websockets, EventSource, prefetch redirects) cannot set
// custom headers; be aware that tokens in URLs are logged by proxies and
// leaked in Referer, so prefer the header transport whenever possible.
var JwtBearerAuth = authentication.AuthType{
	Name:       "JwtBearer",
	Detection:  detection,
	Validation: validate,
}

func detection(w http.ResponseWriter, r *http.Request) bool {
	return detectBearerToken(r) != ""
}

func validate(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	log := logger.GetLogger(r.Context())
	location := detectBearerToken(r)

	var claims map[string]interface{}
	var err error

	switch location {
	case "url":
		log.Info().Msg("Checking authorization from HTTP URL")
		claims, err = validateFromHTTPURL(r)
	case "header":
		log.Info().Msg("Checking authorization from HTTP Header")
		claims, err = validateFromHTTPHeader(r)
	default:
		log.Err(notFoundToken).Send()
		return r, notFoundToken
	}

	if err != nil { // Token is invalid, return unauthorized
		log.Error().Err(err).Msg("Error validating HTTP Authorization")
		return r, err
	}

	if !jwt.IsAudience(claims, jwt.AccessAudience) { // Token isn't an access token, return unauthorized
		return r, errors.New("invalid audience")
	}

	// Add session and claims to context
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

// detectBearerToken reports where the request carries a bearer token: "url"
// (bearer= query param), "header" (Authorization: Bearer ...), or "" if
// neither is present. Malformed or whitespace-only Authorization headers are
// treated as absent instead of panicking on an empty slice index.
func detectBearerToken(r *http.Request) string {
	// Detect URL query param first: this is used by clients that cannot set
	// headers (e.g. browser WebSocket handshakes).
	if r.URL.Query().Get(jwt.TokenHeaderKeyword) != "" {
		return "url"
	}

	authHeader := r.Header.Get(jwt.TokenHeader)
	if authHeader == "" {
		return ""
	}
	authHeaderParts := strings.Fields(authHeader)
	if len(authHeaderParts) == 0 {
		return ""
	}
	if strings.EqualFold(authHeaderParts[0], jwt.TokenHeaderKeyword) {
		return "header"
	}
	return ""
}

// validateFromHTTPHeader extracts the JWT from the HTTP requests and return the claims or
// any error while parsing the token if it has failed (expired, invalid...)
func validateFromHTTPHeader(r *http.Request) (map[string]interface{}, error) {
	tokenString, err := extractFromHTTPHeader(r)
	if err != nil {
		return nil, err
	}

	claims, err := jwt.ParseTokenString(tokenString)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", jwt.InvalidToken, err.Error())
	}

	return claims, nil
}

// validateFromHTTPURL extracts the JWT from the URL query params and returns
// the claims or any error while parsing (expired, invalid, wrong issuer...).
func validateFromHTTPURL(r *http.Request) (map[string]interface{}, error) {
	tokenString, err := extractFromHTTPURL(r)
	if err != nil {
		return nil, err
	}

	claims, err := jwt.ParseTokenString(tokenString)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", jwt.InvalidToken, err.Error())
	}

	return claims, nil
}

// extractFromHTTPHeader extracts the JWT from the headers of an HTTP request
func extractFromHTTPHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get(jwt.TokenHeader)
	if authHeader == "" {
		return "", nil
	}

	authHeaderParts := strings.Fields(authHeader)
	if len(authHeaderParts) != jwt.TokenHeaderParts || !strings.EqualFold(authHeaderParts[0], jwt.TokenHeaderKeyword) {
		return "", illFormedTokenHeader
	}

	return authHeaderParts[jwt.TokenHeaderLocation], nil
}

// extractFromHTTPURL extracts the JWT from the URL query params of an HTTP request
func extractFromHTTPURL(r *http.Request) (string, error) {
	tokenString := r.URL.Query().Get(jwt.TokenHeaderKeyword)
	if tokenString == "" {
		return "", notFoundToken
	}
	return tokenString, nil
}
