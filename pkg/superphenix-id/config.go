package superphenixId

var (
	frameworkPrefix    = "spx"
	effectiveIdContext = "EffectiveId"
)

func SetFrameworkPrefix(prefix string) {
	frameworkPrefix = prefix
}

func SetEffectiveIdContext(contextName string) {
	effectiveIdContext = contextName
}

func FrameworkPrefix() string {
	return frameworkPrefix
}

func EffectiveIdContext() string {
	return effectiveIdContext
}

func Configure(prefix, contextName string) {
	SetFrameworkPrefix(prefix)
	SetEffectiveIdContext(contextName)
}
