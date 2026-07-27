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

// JwtBearerAuth use Superphenix API JWT in Authorization Header or in Url query params
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

	if location == "url" {
		log.Info().Msg("Checking authorization from HTTP URL")
		claims, err = validateFromHTTPURL(r)
	} else if location == "header" {
		log.Info().Msg("Checking authorization from HTTP Header")
		claims, err = validateFromHTTPHeader(r)
	} else {
		// Token not found throw Unauthorized
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

func detectBearerToken(r *http.Request) string {
	// Detect Url Query Param Bearer
	tokenString := r.URL.Query().Get(jwt.TokenHeaderKeyword)
	if tokenString != "" {
		return "url"
	}

	// Detect Authorization Header
	authHeader := r.Header.Get(jwt.TokenHeader)
	if authHeader != "" {
		authHeaderParts := strings.Fields(authHeader)
		// If the authorization header start with bearer
		if strings.ToLower(authHeaderParts[0]) == strings.ToLower(jwt.TokenHeaderKeyword) {
			return "header"
		}
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

// validateFromHTTPURL extracts the JWT from the HTTP requests and return the claims or
// any error while parsing the token if it has failed (expired, invalid...)
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
	if len(authHeaderParts) != jwt.TokenHeaderParts || strings.ToLower(authHeaderParts[0]) != jwt.TokenHeaderKeyword {
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
