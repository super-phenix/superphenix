package apiToken

import (
	cryptoRand "crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	apiToken "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/api-token"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	httpModel "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/sha3"
)

type createBody struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

type createResponse struct {
	Token string `json:"token"`
}

// CreateAPIToken creates a new API token for the authenticated user.
//
//	@Summary		Create an API Token
//	@Description	Create a new API token for the authenticated user. The plain-text token is returned only once in the response.
//	@Tags			api-token
//	@Accept			json
//	@Produce		json
//	@Param			body	body		createBody		true	"Token creation payload"
//	@Success		201		{object}	createResponse	"Created token (plain-text, shown only once)"
//	@Failure		400		{object}	string			"Invalid request (bad format or expired date)"
//	@Failure		401		{object}	string			"Unauthorized"
//	@Failure		500		{object}	string			"Internal server error"
//	@Router			/api-token [post]
//	@Security		Bearer
func (h *Service) CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	var apiTokenCreate createBody
	if err := decoder.HandleHTTPJSON(w, r, &apiTokenCreate, 1); err != nil {
		return
	}

	userUUID, err := fetchUser(r)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get userId")
		httpError.Http(w, r, http.StatusUnauthorized).Msg(http.StatusText(http.StatusUnauthorized))
		return
	}

	var expiresAt time.Time
	if apiTokenCreate.ExpiresAt != "" {
		expiresAt, err = time.Parse(time.RFC3339, apiTokenCreate.ExpiresAt)
		if err != nil {
			log.Error().Err(err).Str("expiresAt", apiTokenCreate.ExpiresAt).Msg("Failed to parse expiresAt")
			httpError.Http(w, r, http.StatusBadRequest).Msg("Invalid expires_at format (RFC3339 expected)")
			return
		}

		if expiresAt.Before(time.Now()) {
			httpError.Http(w, r, http.StatusBadRequest).Msg("ExpiresAt must be in the future")
			return
		}
	}

	prefix, salt, tokenString, err := generateToken()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to generate token")
		return
	}

	token := model.ApiToken{
		Name:           apiTokenCreate.Name,
		UserId:         userUUID,
		Prefix:         prefix,
		Salt:           salt,
		TokenEncrypted: hashToken(tokenString, salt),
		ExpiresAt:      expiresAt,
	}

	if _, err := apiToken.Save(token); err != nil {
		log.Error().Err(err).Msg("Failed to save api token")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to save api token")
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(createResponse{Token: tokenString})
}

// RevokeAPIToken deletes an API token owned by the authenticated user.
//
//	@Summary		Revoke an API Token
//	@Description	Revoke (delete) an existing API token. The token must belong to the authenticated user.
//	@Tags			api-token
//	@Produce		json
//	@Param			tokenId	path	string	true	"Token ID (UUID)"
//	@Success		204		"Token revoked successfully"
//	@Failure		400		{object}	string	"Invalid tokenId format"
//	@Failure		401		{object}	string	"Unauthorized"
//	@Failure		404		{object}	string	"Token not found"
//	@Failure		500		{object}	string	"Internal server error"
//	@Router			/api-token/{tokenId} [delete]
//	@Security		Bearer
func (h *Service) RevokeAPIToken(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	tokenIdStr := chi.URLParam(r, "tokenId")
	tokenId, err := uuid.Parse(tokenIdStr)
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("Invalid tokenId format")
		return
	}

	userUUID, err := fetchUser(r)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get userId")
		httpError.Http(w, r, http.StatusUnauthorized).Msg(http.StatusText(http.StatusUnauthorized))
		return
	}

	token, err := apiToken.FindById(tokenId)
	if err != nil {
		log.Error().Err(err).Str("tokenId", tokenIdStr).Msg("Failed to find api token")
		httpError.Http(w, r, http.StatusNotFound).Msg("Api token not found")
		return
	}

	if token.UserId != userUUID {
		log.Warn().Str("userId", userUUID.String()).Str("tokenOwner", token.UserId.String()).Str("tokenId", tokenIdStr).Msg("User tried to revoke a token they don't own")
		httpError.Http(w, r, http.StatusNotFound).Msg("Api token not found")
		return
	}

	err = apiToken.DeleteById(tokenId)
	if err != nil {
		log.Error().Err(err).Str("tokenId", tokenIdStr).Msg("Failed to delete api token")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to delete api token")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListAPIToken returns all API tokens for the authenticated user.
//
//	@Summary		List API Tokens
//	@Description	Retrieve all API tokens belonging to the authenticated user. Token secrets are not included.
//	@Tags			api-token
//	@Produce		json
//	@Success		200	{array}		model.APIApiToken	"List of API tokens"
//	@Failure		400	{object}	string				"Invalid userId format"
//	@Failure		401	{object}	string				"Unauthorized"
//	@Failure		500	{object}	string				"Internal server error"
//	@Router			/api-token [get]
//	@Security		Bearer
func (h *Service) ListAPIToken(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())

	userUUID, err := fetchUser(r)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get userId")
		httpError.Http(w, r, http.StatusUnauthorized).Msg(http.StatusText(http.StatusUnauthorized))
		return
	}

	tokens, err := apiToken.FindAllByUser(userUUID)
	if err != nil {
		log.Error().Err(err).Str("userId", userUUID.String()).Msg("Failed to fetch api tokens")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to fetch api tokens")
		return
	}

	if tokens == nil {
		tokens = []httpModel.APIApiToken{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tokens)
}

// hashToken hashes a token using SHA3-512 and a salt, and returns the hex string representation.
func hashToken(token string, salt string) string {
	hash := sha3.New512()
	hash.Write([]byte(fmt.Sprintf("%s:%s", token, salt)))
	return hex.EncodeToString(hash.Sum(nil))
}

// generateToken generates a random token string and returns its prefix, a salt and the full token.
func generateToken() (string, string, string, error) {
	prefixBytes := make([]byte, 4)
	if _, err := cryptoRand.Read(prefixBytes); err != nil {
		return "", "", "", err
	}
	prefix := hex.EncodeToString(prefixBytes)

	saltBytes := make([]byte, 16)
	if _, err := cryptoRand.Read(saltBytes); err != nil {
		return "", "", "", err
	}
	salt := hex.EncodeToString(saltBytes)

	tokenBytes := make([]byte, 28) // 56 characters in hex
	if _, err := cryptoRand.Read(tokenBytes); err != nil {
		return "", "", "", err
	}
	token := prefix + hex.EncodeToString(tokenBytes)

	return prefix, salt, token, nil
}

// verifyToken verifies a token against its SHA3-512 hashed version and salt using constant-time comparison.
func verifyToken(token, salt, hashedToken string) bool {
	return subtle.ConstantTimeCompare([]byte(hashToken(token, salt)), []byte(hashedToken)) == 1
}

func fetchUser(r *http.Request) (uuid.UUID, error) {
	log := logger.GetLogger(r.Context())
	userId := r.Context().Value(consts.ContextUserId)
	if userId == nil {
		return uuid.Nil, fmt.Errorf("userId not found in context")
	}

	userIdStr, ok := userId.(string)
	if !ok {
		return uuid.Nil, fmt.Errorf("userId in context is not a string")
	}

	userUUID, err := uuid.Parse(userIdStr)
	if err != nil {
		log.Error().Err(err).Str("userId", userIdStr).Msg("Failed to parse userId")
		return uuid.Nil, fmt.Errorf("invalid userId format")
	}

	return userUUID, nil
}
