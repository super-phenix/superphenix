package user

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/user"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/utils"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/google/uuid"
)

type RegenerateInviteCodeResponse struct {
	InviteCode uuid.UUID `json:"inviteCode"`
}

// RegenerateInviteCode
//
//	@Summary		Regenerate Invite Code
//	@Description	Regenerate the invite code of the authenticated user
//	@Tags			v1, user
//	@Produce		json
//	@Success		200	{object}	RegenerateInviteCodeResponse	"New Invite Code"
//	@Failure		401
//	@Failure		500
//	@Router			/v1/invite-code [post]
//	@Security		Bearer
func (h *Service) RegenerateInviteCode(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())

	userUuid, err := utils.GetUserUuid(r)
	if err != nil {
		log.Err(err).Msg("Failed to get user id from context")
		httpError.Http(w, r, http.StatusUnauthorized).Msg("Unauthorized")
		return
	}

	newCode, err := user.RegenerateInviteCode(userUuid)
	if err != nil {
		if errors.Is(err, user.ErrInviteCodeCooldown) {
			httpError.Http(w, r, http.StatusTooManyRequests).Msg(err.Error())
			return
		}
		log.Err(err).Str("userId", userUuid.String()).Msg("Failed to regenerate invite code")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to regenerate invite code")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(RegenerateInviteCodeResponse{
		InviteCode: newCode,
	})
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling response")
	}
	_, _ = w.Write(marshal)
}
