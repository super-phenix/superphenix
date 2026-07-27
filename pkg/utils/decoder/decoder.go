package decoder

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	BadlyFormedJSON = errors.New("JSON is badly formed")
)

// DecodeJSON decodes the JSON passed through the 'reader' into the structure passed through the 'destination' variable.
// The function returns errors of type:
//   - BadlyFormedJSON if there are errors in the JSON and we can't decode it
//   - Undefined if the decoder failed to work, and we return a generic error describing what happened
//
// BadlyFormedJSON are emitted if the user is supplying faulty JSON and should result in error codes putting the  user at fault
// Any other error was probably generated on our side and should tell the user the processing failed here
func DecodeJSON(reader io.ReadCloser, destination interface{}) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields() // Do not allow extra fields in the JSON not present in our struct

	if err := decoder.Decode(&destination); err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("error at position %d", syntaxError.Offset)
			return fmt.Errorf("%w: %s", BadlyFormedJSON, msg)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return BadlyFormedJSON

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return fmt.Errorf("%w: %s", BadlyFormedJSON, msg)

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("unknown field %s", fieldName)
			return fmt.Errorf("%w: %s", BadlyFormedJSON, msg)

		case errors.Is(err, io.EOF):
			msg := "JSON must not be empty"
			return fmt.Errorf("%w: %s", BadlyFormedJSON, msg)

		default:
			return err
		}
	}

	// If we decode again and there's still data, there's multiple JSON objects
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		msg := "must only contain a single JSON object"
		return fmt.Errorf("%w: %s", BadlyFormedJSON, msg)
	}

	return nil
}
