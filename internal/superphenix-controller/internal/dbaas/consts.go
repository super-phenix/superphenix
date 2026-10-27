package dbaas

const (
	// RestartAnnotation triggers a rolling restart of the instances, as kubectl cnpg restart does.
	RestartAnnotation = "kubectl.kubernetes.io/restartedAt"

	// AppSecretSuffix names the Secret CloudNative-PG creates for the application user.
	AppSecretSuffix = "-app"

	// LBLocalIDPrefix prefixes the database localID in the localID of its SwitchLBRule.
	LBLocalIDPrefix = "lb."
)

// proxyPortRoles maps the sfs-dbaas proxy ports to the CloudNative-PG service they serve.
var proxyPortRoles = map[int64]string{
	5432: "rw",
	5433: "ro",
	5434: "r",
}
