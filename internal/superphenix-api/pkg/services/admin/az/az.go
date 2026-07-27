package az

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/proxy"

	"github.com/rs/zerolog/log"
)

// FullHealth check health of all AZs
//
//	@Summary		Check health of all AZs
//	@Description	Contact each configured AZ's /health endpoint to determine availability
//	@Tags			Admin endpoint
//	@Produce		json
//	@Success		200	{object}	map[string]string	"Map of AZ code to status (up/down)"
//	@Failure		500	{string}	string				"Error"
//
//	@Router			/admin/az/health [get]
func (h *Service) FullHealth(w http.ResponseWriter, r *http.Request) {
	result := make(map[string]string)

	for _, a := range h.cfg.AZs {
		resp, err := proxy.SendRequest(r.Context(), a.ControllerUrl+"/health", http.MethodGet, http.NoBody, h.cfg.Controller.AuthSecret)
		if err != nil || resp.StatusCode != http.StatusOK {
			result[a.Code] = "down"
		} else {
			result[a.Code] = "up"
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Error().Err(err).Msg("Failed to encode health result")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
