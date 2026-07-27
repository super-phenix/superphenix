package decoder

import (
	"errors"
	"fmt"
	"net/http"

	logger "github.com/super-phenix/superphenix/pkg/utils/log"
	"github.com/super-phenix/superphenix/pkg/utils/validation"
)

const (
	bytesInMegabytes = 1024 * 1024
)

type HTTPJSONDecodeError struct {
	Status  int
	Message string
}

func (e *HTTPJSONDecodeError) Error() string {
	return e.Message
}

// DecodeJSONFromHTTP decodes the body of an HTTP request and returns a custom error indicating the
// HTTP return code that should be forwarded to the remote client if the decoding fails
// The resulting structure decoded from the JSON is placed into the 'destination' parameter
// The maximum size of the JSON before the decoder returns an error is set using the 'mbMaxSize' parameter
func DecodeJSONFromHTTP(w http.ResponseWriter, r *http.Request, destination interface{}, mbMaxSize uint) error {
	// Check header indicates the client is sending JSON
	if r.Header.Get("Content-Type") != "" {
		if r.Header.Get("Content-Type") != "application/json" {
			return &HTTPJSONDecodeError{
				Status:  http.StatusUnsupportedMediaType,
				Message: "content-Type header is not application/json",
			}
		}
	}

	// Set maximum readable size and decode the JSON
	r.Body = http.MaxBytesReader(w, r.Body, int64(mbMaxSize)*bytesInMegabytes)

	if err := DecodeJSON(r.Body, &destination); err != nil {
		switch {
		case errors.Is(err, BadlyFormedJSON):
			return &HTTPJSONDecodeError{
				Status:  http.StatusBadRequest,
				Message: err.Error(),
			}
		case err.Error() == "http: request body too large":
			return &HTTPJSONDecodeError{
				Status:  http.StatusRequestEntityTooLarge,
				Message: fmt.Sprintf("request body must not be larger than %dMB", mbMaxSize),
			}
		default:
			return &HTTPJSONDecodeError{
				Status:  http.StatusInternalServerError,
				Message: err.Error(),
			}
		}
	}

	return nil
}

// HandleHTTPJSON decodes the incoming JSON from the body of the HTTP request into the 'destination' parameter
// It handles any error arising from the decoding process by sending a human-readable message and
// a corresponding HTTP status to the remote client
// The maximum size of the incoming JSON can be constrained using the 'mbMaxSize' parameter, in megabytes
// If an error is produced, it is also returned by the function so that the handler can abort the processing
// of the HTTP request immediately
func HandleHTTPJSON(w http.ResponseWriter, r *http.Request, destination interface{}, mbMaxSize uint) error {
	log := logger.GetLogger(r.Context())

	if err := DecodeJSONFromHTTP(w, r, &destination, mbMaxSize); err != nil {
		log.Error().Err(err).Msg("Failed to decode json")
		httpError := err.(*HTTPJSONDecodeError)
		http.Error(w, err.Error(), httpError.Status)
		return err
	}

	if err := validation.HTTPV1Validator.ValidateStruct(destination); err != nil {
		log.Error().Err(err).Str("validator_json", err.JSON()).Msg("Validation failure")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.JSON()))
		return err
	}

	return nil
}
