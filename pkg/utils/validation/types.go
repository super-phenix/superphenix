package validation

import (
	"encoding/json"
	"errors"
)

var (
	FailedToValidate = errors.New("failed to run validation")
	InvalidStructure = errors.New("structure is not valid")
)

// StructureErrorV1 describes an error during the validation process of a structure
type StructureErrorV1 struct {
	ErrorType   error                 // What type of error was generated during the validation
	FieldErrors []InvalidFieldErrorV1 // Detail of the error of each field
}

// InvalidFieldErrorV1 describes an error during validation of a field in a structure and helps debug the source
// of the problem by exposing a few indications on what went wrong
type InvalidFieldErrorV1 struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Kind    string `json:"kind"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	Param   string `json:"param"`
	Message string `json:"message"`
}

func (e *StructureErrorV1) Error() string {
	return e.ErrorType.Error()
}

func (e *StructureErrorV1) JSON() string {
	prettyStruct := struct {
		ErrorType   string                `json:"error"`
		FieldErrors []InvalidFieldErrorV1 `json:"fieldErrors"`
	}{e.ErrorType.Error(), e.FieldErrors}

	indented, err := json.MarshalIndent(prettyStruct, "", "  ")
	if err != nil {
		return ""
	}

	return string(indented)
}
