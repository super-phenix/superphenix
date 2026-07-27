package validation

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// ValidateStruct runs validation of a structure and returns an error struct detailing what failed
// If the validation was successful, nil is returned.
func (v *ValidatorV1) ValidateStruct(structure any) *StructureErrorV1 {
	err := v.validator.Struct(structure)
	validationError := &StructureErrorV1{}

	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			validationError.ErrorType = fmt.Errorf("%w : %s", FailedToValidate, err.Error())
			return validationError
		}

		var errors []InvalidFieldErrorV1
		for _, err := range err.(validator.ValidationErrors) {
			fieldError := InvalidFieldErrorV1{
				Field:   err.Field(),
				Tag:     err.Tag(),
				Kind:    fmt.Sprintf("%v", err.Kind()),
				Type:    fmt.Sprintf("%v", err.Type()),
				Value:   fmt.Sprintf("%v", err.Value()),
				Param:   err.Param(),
				Message: err.Error(),
			}

			errors = append(errors, fieldError)
		}

		validationError.ErrorType = InvalidStructure
		validationError.FieldErrors = errors
		return validationError
	}

	return nil
}
