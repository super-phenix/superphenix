package az

import (
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/az"
	httpModel "github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/proxy"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/go-chi/chi/v5"
)

// ListAZs return the list of all configured AZs
//
//	@Summary		List AZ
//	@Description	List AZ from configuration
//	@Tags			v1, az
//	@Produce		json
//	@Param			orgaId	path	string		true	"Organization ID"
//	@Success		200		{array}	model.APIAz	"AZ List"
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/az-list [get]
//	@Security		Bearer[OrganizationRead]
func (h *Service) ListAZs(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")

	azConfigs := az.FindAll(orgaId)

	var result []httpModel.APIAz
	for _, a := range azConfigs {
		result = append(result, httpModel.APIAz{
			Code:    a.Code,
			Name:    a.Name,
			LogoUrl: a.LogoUrl,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	marshal, err := json.Marshal(result)
	if err != nil {
		log.Error().Err(err).Msg("Error marshalling response")
	}
	_, _ = w.Write(marshal)
}

// StatusCheck check health of all AZs available for the user
//
//	@Summary		Check health of AZs
//	@Description	Fetch all available for the user and contact each AZ's /health endpoint to determine availability
//	@Tags			v1, az
//	@Produce		json
//	@Success		200	{object}	map[string]string	"Map of AZ code to status (up/down)"
//	@Failure		500	{string}	string				"Error"
//
//	@Router			/v1/organization/{orgaId}/az-status [get]
func (h *Service) StatusCheck(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())
	orgaId := chi.URLParam(r, "orgaId")

	azConfigs := az.FindAll(orgaId)
	result := make(map[string]string)

	for _, a := range azConfigs {
		resp, err := proxy.SendRequest(r.Context(), a.ControllerUrl+"/health", http.MethodGet, http.NoBody, a.AuthSecret)
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
