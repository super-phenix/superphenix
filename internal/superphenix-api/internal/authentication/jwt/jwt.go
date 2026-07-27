package jwt

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"

	"github.com/golang-jwt/jwt/v4"
)

const (
	ClaimsCtxKey        string = "JWT_CLAIMS"
	TokenHeader         string = "Authorization"
	TokenHeaderKeyword  string = "bearer"
	TokenHeaderParts    int    = 2
	TokenHeaderLocation int    = 1

	AccessAudience  = "access"
	RefreshAudience = "refresh"

	AccessToken  = "access_token"
	RefreshToken = "refresh_token"
)

var (
	TokenIssuer  = config.Global.Authentication.JwtIssuer
	Secret       = []byte(config.Global.Authentication.JwtSecret)
	InvalidToken = errors.New("token is invalid")
)

func CreateAccessToken(validityDuration time.Duration, subjectID string) (string, error) {
	token := NewJWTWithSubjectID(validityDuration, subjectID)
	jwtString, err := token.SignedString(Secret)

	if err != nil {
		return "", err
	}

	return jwtString, nil
}

func CreateAccessAndRefreshTokens(validityAccessDuration, validityRefreshDuration time.Duration, subjectID string) (map[string]string, error) {
	accessToken := NewJWTWithSubjectID(validityAccessDuration, subjectID)
	t, err := accessToken.SignedString(Secret)
	if err != nil {
		return nil, err
	}

	refreshToken := NewRefreshToken(validityRefreshDuration, subjectID)
	rt, err := refreshToken.SignedString(Secret)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		AccessToken:  t,
		RefreshToken: rt,
	}, nil
}

// NewJWTWithSubjectID generates a new JWT access token with a claim specifying the subject ID
// and with a custom validity duration
func NewJWTWithSubjectID(validityDuration time.Duration, subjectID string) *jwt.Token {
	claims := jwt.RegisteredClaims{
		Issuer:    TokenIssuer,
		Subject:   subjectID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(validityDuration)),
		NotBefore: jwt.NewNumericDate(time.Now()),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Audience:  jwt.ClaimStrings{AccessAudience},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
}

// NewRefreshToken generates a new JWT refresh token with a claim specifying the subject ID
// and with a custom validity duration
func NewRefreshToken(validityDuration time.Duration, subjectID string) *jwt.Token {
	claims := jwt.RegisteredClaims{
		Issuer:    TokenIssuer,
		Subject:   subjectID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(validityDuration)),
		NotBefore: jwt.NewNumericDate(time.Now()),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Audience:  jwt.ClaimStrings{RefreshAudience},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
}

// UpdateRequestContext adds the claims of a JWT to the context of the initial request
func UpdateRequestContext(ctx context.Context, claims jwt.MapClaims) context.Context {
	ctx = context.WithValue(ctx, ClaimsCtxKey, claims)
	ctx = context.WithValue(ctx, consts.ContextSessionId, claims["sub"])
	return ctx
}

// ParseTokenString parses a JWT string and returns the claims contained in it and any error
// if parsing has failed (expired, invalid...)
func ParseTokenString(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return Secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token/invalid claims")
}

func IsAudience(claims jwt.MapClaims, audience string) bool {
	return claims.VerifyAudience(audience, true)
}
