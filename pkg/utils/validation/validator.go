package validation

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

type Validator interface {
	ValidateStruct(structure any) interface{}
}

type ValidatorV1 struct {
	validator *validator.Validate
}

func GetValidatorV1() ValidatorV1 {
	return ValidatorV1{
		validator: validator.New(),
	}
}

// interpretTagsFromJson makes the validator override the name of the structure field by its JSON name
func (v *ValidatorV1) interpretTagsFromJson() {
	v.validator.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}
