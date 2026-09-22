package validation

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

// MACRegex validates IEEE 802 colon notation (e.g. 52:54:00:11:22:33).
var MACRegex = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$`)

// IsValidMAC returns true if mac conforms to standard IEEE 802 colon notation.
func IsValidMAC(mac string) bool {
	return MACRegex.MatchString(mac)
}

func validateMAC(fl validator.FieldLevel) bool {
	return IsValidMAC(fl.Field().String())
}

type Validator interface {
	ValidateStruct(structure any) interface{}
}

type ValidatorV1 struct {
	validator *validator.Validate
}

func GetValidatorV1() ValidatorV1 {
	v := validator.New()
	_ = v.RegisterValidation("mac", validateMAC)
	return ValidatorV1{
		validator: v,
	}
}

// Validator returns the underlying validator.Validate instance
func (v ValidatorV1) Validator() *validator.Validate {
	return v.validator
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
