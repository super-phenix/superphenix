package httpError

import (
	"encoding/json"
	"io"
	"net/http"

	logger "github.com/super-phenix/superphenix/pkg/utils/log"
)

func RetrieveHttpError(resp *http.Response) *ErrorBody {
	log := logger.GetLogger(resp.Request.Context())
	// read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Error reading response body")
		return nil
	}
	// close it right after we don't need it
	if err := resp.Body.Close(); err != nil {
		log.Error().Err(err).Msg("Error closing response body")
	}
	var res ErrorBody
	if err := json.Unmarshal(body, &res); err != nil {
		log.Error().Err(err).Str("body", string(body)).Str("request-url", resp.Request.URL.String()).Msg("Failed to unmarshal")
		return nil
	}
	return &res
}
