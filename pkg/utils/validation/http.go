package validation

var (
	HTTPV1Validator ValidatorV1
)

func init() {
	initHTTPValidatorV1()
}

func initHTTPValidatorV1() {
	HTTPV1Validator = GetValidatorV1()
	HTTPV1Validator.interpretTagsFromJson()
}
