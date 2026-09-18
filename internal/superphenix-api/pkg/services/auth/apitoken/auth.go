package apiToken

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	apiToken "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/api-token"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/google/uuid"
)

var (
	illFormedTokenHeader = errors.New("authorization header format must be api-token {token}")
)

const (
	tokenHeader         string = "Authorization"
	tokenHeaderKeyword  string = "api-token"
	tokenHeaderParts    int    = 2
	tokenHeaderLocation int    = 1
)

// ApiTokenAuth use Superphenix API Api Token in Authorization Header
var ApiTokenAuth = authentication.AuthType{
	Name:       "ApiTokenAuth",
	Detection:  detection,
	Validation: validate,
}

func detection(w http.ResponseWriter, r *http.Request) bool {
	// Detect Authorization Header
	authHeader := r.Header.Get(tokenHeader)
	if authHeader == "" {
		return false
	}
	authHeaderParts := strings.Fields(authHeader)
	if len(authHeaderParts) == 0 {
		return false
	}
	return strings.EqualFold(authHeaderParts[0], tokenHeaderKeyword)
}

func validate(w http.ResponseWriter, r *http.Request) (*http.Request, error) {
	log := logger.GetLogger(r.Context())

	token, err := extractFromHTTPHeader(r)
	if err != nil {
		// Token is invalid, return unauthorized
		log.Error().Err(err).Msg("Error validating Api Token Authorization")
		return r, err
	}

	// Token format is expected to be prefix (8 chars) + random part
	if len(token) < 8 {
		log.Error().Msg("API Token too short")
		return r, errors.New("API Token invalid")
	}

	prefix := token[:8]
	userTokens, err := apiToken.FindAllByTokenPrefix(prefix)
	if err != nil {
		log.Error().Err(err).Str("prefix", prefix).Msg("Failed to find the token in db")
		return r, err
	}

	var authenticatedToken *model.ApiToken
	for _, ut := range userTokens {
		// Verify the token with salt
		if verifyToken(token, ut.Salt, ut.TokenEncrypted) {
			authenticatedToken = &ut
			break
		}
	}

	if authenticatedToken == nil {
		log.Error().Msg("API Token hash mismatch")
		return r, errors.New("API Token invalid")
	}

	// Check if the user is active
	if authenticatedToken.User.ID == uuid.Nil || authenticatedToken.User.DeletedAt.Valid || !authenticatedToken.User.IsActive {
		log.Error().Any("token", authenticatedToken).Msg("API Token invalid")
		return r, errors.New("API Token invalid")
	}

	// Check if the api key is expired
	if !authenticatedToken.ExpiresAt.IsZero() && authenticatedToken.ExpiresAt.Before(time.Now()) {
		log.Error().Any("token", authenticatedToken).Msg("API Token expired")
		return r, errors.New("API Token expired")
	}

	// Add the userId into the request context
	ctx := context.WithValue(r.Context(), consts.ContextUserId, authenticatedToken.UserId.String())
	r = r.WithContext(ctx)
	return r, nil
}

// extractFromHTTPHeader extracts the api token from the headers of an HTTP request
func extractFromHTTPHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get(tokenHeader)
	if authHeader == "" {
		return "", nil
	}

	authHeaderParts := strings.Fields(authHeader)
	if len(authHeaderParts) != tokenHeaderParts || strings.ToLower(authHeaderParts[0]) != tokenHeaderKeyword {
		return "", illFormedTokenHeader
	}

	return authHeaderParts[tokenHeaderLocation], nil
}
