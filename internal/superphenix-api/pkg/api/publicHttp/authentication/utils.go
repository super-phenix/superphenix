package authentication

import (
	"context"
	"fmt"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/user"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

func RetrieveUserFromSession(r *http.Request) (*http.Request, error) {
	log := logger.GetLogger(r.Context())
	sessionId := r.Context().Value(consts.ContextSessionId)
	if sessionId != nil {

		result, err := user.FindByProviderId(sessionId.(string), db.KratosProvider)
		if err != nil {
			log.Error().Ctx(r.Context()).Err(err).Msg("Error finding user")
			return r, err
		}

		active, err := user.IsUserActive(result.ID.String())
		if err != nil {
			log.Error().Err(err).Msg("Error checking if user is active")
			return r, err
		}

		if !active {
			log.Debug().Str("userId", result.ID.String()).Msg("User is not active")
			return r, fmt.Errorf("user is not active")
		}

		r = r.WithContext(context.WithValue(r.Context(), consts.ContextUserId, result.ID.String()))
		return r, nil
	}

	log.Debug().Ctx(r.Context()).Msg("No session found")
	return r, fmt.Errorf("no session found")
}
