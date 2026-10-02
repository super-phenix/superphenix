package config

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/viper"
)

const (
	AppName   = "superphenix-api" // Name of the application in lowercase, used to determine the configuration path
	ApiPrefix = "/api/spx-ctrl"
)

// keyDelimiter separates viper key paths. "." (the viper default) would split
// the version keys of kubernetes.versions (e.g. "0.8.0") into nested maps.
const keyDelimiter = "|"

var (
	FileNotFound = errors.New("couldn't find configuration file")

	// ErrInvalidSPXVersion is returned when an AZ SPX version is not a semver.
	ErrInvalidSPXVersion = errors.New("invalid SPX version")
	// ErrNoKaasProfile is returned when no KaaS profile applies to an AZ SPX version.
	ErrNoKaasProfile = errors.New("no KaaS configuration for this SPX version")
)

type RepoArgoAppConfig struct {
	RepoURL        string `yaml:"repoURL"`
	TargetRevision string `yaml:"targetRevision"`
	Chart          string `yaml:"chart,omitempty"`
	Path           string `yaml:"path,omitempty"`
}

// KubeVersionConfig is a supported Kubernetes version with an optional chart
// repo override and an optional FQDN-rewrite flag. When Repo is set it fully
// replaces the default KaaS repo. Fqdn nil means true (the default for >=1.35
// and newer versions); legacy <1.35 versions set fqdn:false.
type KubeVersionConfig struct {
	Version string             `yaml:"version"`
	Repo    *RepoArgoAppConfig `yaml:"repo,omitempty"`
	Fqdn    *bool              `yaml:"fqdn"`
}

// AzDomainConfig holds the URLs used in KaaS cluster configuration for one AZ.
// Internal is the host the in-cluster kube-API proxy resolves to reach control
// planes, including across AZs during disaster recovery. External is a URL
// template for the control plane FQDN, where "%s" is replaced by the cluster ID.
type AzDomainConfig struct {
	Internal string `yaml:"internal"`
	External string `yaml:"external"`
}

// KaasAzDomain returns the azDomains entry for az. ok is false for string
// entries and missing keys.
func (c *Config) KaasAzDomain(az string) (AzDomainConfig, bool) {
	entry, ok := c.ProductsConfig.ArgoApp.Kubernetes.AzDomains[az].(map[string]any)
	if !ok {
		return AzDomainConfig{}, false
	}
	internal, _ := entry["internal"].(string)
	external, _ := entry["external"].(string)
	return AzDomainConfig{Internal: internal, External: external}, true
}

// KaasVersionProfile is the KaaS configuration applying to AZs whose SPX version
// is greater than or equal to its key in KubernetesArgoAppConfig.Versions.
type KaasVersionProfile struct {
	Repo         RepoArgoAppConfig   `yaml:"repo"`
	KubeVersions []KubeVersionConfig `yaml:"kubeVersions"`
}

// KubernetesArgoAppConfig is the KaaS product configuration.
type KubernetesArgoAppConfig struct {
	// Versions maps a minimum SPX version to the KaaS profile of the AZs
	// running that version or a newer one, up to the next key.
	Versions map[string]KaasVersionProfile `yaml:"versions"`
	// sfs-kaas `azDomains` value. Entry shapes:
	//   <az code>: {internal, external}  sfs-kaas >= 0.7.0
	//   <region>: <domain>               sfs-kaas < 0.7.0
	AzDomains map[string]any `yaml:"azDomains"`
}

// ProfileFor returns the profile with the highest minimum version lower than or
// equal to spxVersion. Pre-release tags are ignored.
func (k KubernetesArgoAppConfig) ProfileFor(spxVersion string) (KaasVersionProfile, error) {
	version, err := parseSPXVersion(spxVersion)
	if err != nil {
		return KaasVersionProfile{}, fmt.Errorf("%w %q: %s", ErrInvalidSPXVersion, spxVersion, err.Error())
	}

	var (
		best    *semver.Version
		profile KaasVersionProfile
	)
	for key, p := range k.Versions {
		minVersion, err := parseSPXVersion(key)
		if err != nil || minVersion.GreaterThan(version) {
			continue
		}
		if best == nil || minVersion.GreaterThan(best) {
			best, profile = minVersion, p
		}
	}
	if best == nil {
		return KaasVersionProfile{}, fmt.Errorf("%w %q", ErrNoKaasProfile, spxVersion)
	}
	return profile, nil
}

// parseSPXVersion parses a semver, accepting a leading "v" and dropping the
// pre-release and metadata parts.
func parseSPXVersion(raw string) (*semver.Version, error) {
	v, err := semver.NewVersion(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	return semver.New(v.Major(), v.Minor(), v.Patch(), "", ""), nil
}

// validate returns the configuration errors of the KaaS profiles.
func (k KubernetesArgoAppConfig) validate() []string {
	if len(k.Versions) == 0 {
		return []string{"productsConfig.argoApp.kubernetes.versions must not be empty"}
	}

	var errs []string
	seen := make(map[string]string, len(k.Versions))
	for key, p := range k.Versions {
		prefix := fmt.Sprintf("productsConfig.argoApp.kubernetes.versions[%q]", key)
		v, err := parseSPXVersion(key)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: key must be a semantic version: %s", prefix, err.Error()))
		} else if other, ok := seen[v.String()]; ok {
			errs = append(errs, fmt.Sprintf("%s: same version as %q", prefix, other))
		} else {
			seen[v.String()] = key
		}
		if p.Repo.RepoURL == "" || p.Repo.TargetRevision == "" {
			errs = append(errs, prefix+".repo must set repoURL and targetRevision")
		}
		if len(p.KubeVersions) == 0 {
			errs = append(errs, prefix+".kubeVersions must not be empty")
		}
	}
	sort.Strings(errs)
	return errs
}

// ResolveKubeVersionRepo returns the effective repo for version and whether the
// version is supported. A version's own Repo fully replaces def.
func ResolveKubeVersionRepo(versions []KubeVersionConfig, def RepoArgoAppConfig, version string) (RepoArgoAppConfig, bool) {
	for _, v := range versions {
		if v.Version == version {
			if v.Repo != nil {
				return *v.Repo, true
			}
			return def, true
		}
	}
	return RepoArgoAppConfig{}, false
}

// AZConfig represents an Availability Zone defined in configuration
type AZConfig struct {
	Code          string `yaml:"-"`
	AuthSecret    string `yaml:"authSecret"`
	Name          string `yaml:"name"`
	LogoUrl       string `yaml:"logoUrl"`
	ControllerUrl string `yaml:"controllerUrl"`
	Destination   string `yaml:"destination"`
	// ClusterName is the name of the operator Cluster CR of the AZ. Empty
	// means Destination, or a lookup by availability zone for "in-cluster".
	ClusterName string   `yaml:"clusterName"`
	Whitelist   []string `yaml:"whitelist"`
}

// Config describes the configuration structure accepted by this application
type Config struct {
	Logging struct {
		Pretty bool
	}

	PublicHTTP struct {
		Address        string
		HealthEndpoint string
		MaxBodySize    uint
	}

	AdminHTTP struct {
		Enabled    bool
		Address    string
		AuthSecret string
	}

	ReadinessProbe struct {
		Enabled bool
		Address string
	} `yaml:"readinessProbe"`

	SuperAdmins []string `yaml:"superAdmins"`

	Tracing struct {
		Enabled        bool
		Address        string
		CleanupTimeout time.Duration
		BatchTimeout   time.Duration
	}

	Metrics struct {
		Enabled  bool
		Endpoint string
		Address  string
		Port     int
	}

	Authentication struct {
		JwtIssuer      string
		JwtSecret      string
		KratosCookie   string
		KratosEndpoint string
	}

	Swagger struct {
		BaseURL string
	}

	Session struct {
		DefaultReturnUrl string
		AccessValidity   time.Duration
		RefreshValidity  time.Duration

		Cookies struct {
			Name     string
			Domain   string
			Path     string
			SameSite string
			Secure   bool
		}

		Cors struct {
			AllowedOrigins []string
			// AllowUnsafeWildcard opts into treating "*" in AllowedOrigins
			// as a real wildcard. Off by default because the middleware sets
			// AllowCredentials=true, and "*" then makes parseReturnUrl an open
			// redirect that leaks tokens. Only enable for local dev / tests.
			AllowUnsafeWildcard bool `yaml:"allowUnsafeWildcard"`
		}
	}

	UserSettings struct {
		UserIsActiveOnCreate           bool          `yaml:"userIsActiveOnCreate"` // If false, need manual activation in database
		EnableProjectDefaultResources  bool          `yaml:"enableProjectDefaultResources"`
		InviteCodeRegenerationCooldown time.Duration `yaml:"inviteCodeRegenerationCooldown"`
	} `yaml:"userSettings"`

	Permify struct {
		Url string
	}

	ArgoController struct {
		// Kubeconfig is an explicit path to a kubeconfig; empty means the
		// default loading rules then in-cluster config.
		Kubeconfig          string `yaml:"kubeconfig"`
		AppProjectNamespace string `yaml:"appProjectNamespace"`

		GarbageCollection struct {
			Enabled      bool          `yaml:"enabled"`
			Interval     time.Duration `yaml:"interval"`
			Timeout      time.Duration `yaml:"timeout"`
			LabelMarkKey string        `yaml:"labelMarkKey"`
			Delay        time.Duration `yaml:"delay"`
			Debug        bool          `yaml:"debug"`
		} `yaml:"garbageCollection"`
	}

	AuditLog AuditLogConfig `yaml:"auditLog"`

	Database struct {
		Host     string
		Port     string
		Username string
		Password string
		Database string
	}

	AZs map[string]AZConfig `yaml:"azs"`

	// Operator locates the operator Cluster CRs, read to get the SPX version
	// of each AZ. Its cluster connection is independent of ArgoController's.
	Operator struct {
		// Kubeconfig is an explicit path to a kubeconfig; empty means the
		// default loading rules then in-cluster config.
		Kubeconfig string `yaml:"kubeconfig"`
		Namespace  string `yaml:"namespace"`
	} `yaml:"operator"`

	SpxPrefix string `yaml:"spxPrefix"`
	ArgoCdUrl string `yaml:"argoCdUrl"`

	ProductsConfig struct {
		ArgoApp struct {
			Kubernetes KubernetesArgoAppConfig `yaml:"kubernetes"`

			Backup struct {
				Repo     RepoArgoAppConfig `yaml:"repo"`
				Schedule struct {
					MinHour int `yaml:"minHour"`
					MaxHour int `yaml:"maxHour"`
				} `yaml:"schedule"`
			} `yaml:"backup"`
		} `yaml:"argoApp"`

		DefaultVPC struct {
			ProductName string `yaml:"productName"`
		} `yaml:"defaultVpc"`

		DefaultSubnet struct {
			ProductName       string `yaml:"productName"`
			Protocol          string `yaml:"protocol"`
			IPv4              string `yaml:"ipv4"`
			IPv6              string `yaml:"ipv6"`
			NatGatewayEnabled bool   `yaml:"natGatewayEnabled"`
			Private           bool   `yaml:"private"`
			DnsV4             string `yaml:"dnsV4"`
			DnsV6             string `yaml:"dnsV6"`
		} `yaml:"defaultSubnet"`

		SnapshotSchedule struct {
			MinHour int `yaml:"minHour"`
			MaxHour int `yaml:"maxHour"`
		} `yaml:"snapshotSchedule"`

		S3 struct {
			// MaxMinifiedJSONLen caps bucket policy/lifecycle JSON minified length
			MaxMinifiedJSONLen int `yaml:"maxMinifiedJSONLen"`
		} `yaml:"s3"`
	} `yaml:"productsConfig"`
}

var defaultConfig = []byte(`
logging:
  pretty: true
publicHttp:
  address: ":8080"
  healthEndpoint: "/healthz"
  maxBodySize: 5
adminHttp:
  enabled: true
  address: ":7000"
  authSecret: ""
readinessProbe:
  enabled: true
  address: ":9000"
superAdmins: []
tracing:
  enabled: false	
  address: "http://<tracing-host>:<tracing-port>/api/traces"
  cleanupTimeout: "5s"
  batchTimeout: "10s"
metrics:
  enabled: true
  endpoint: "/metrics"
  address: ""
  port: 9090
authentication:
  jwtIssuer: "superphenix-api"
  jwtSecret: "secret"
  kratosCookie: "ory_kratos_session"
  kratosEndpoint: "https://<kratos-public-host>"
swagger:
  baseURL: "localhost:8080"
userSettings:
  userIsActiveOnCreate: false
  enableProjectDefaultResources: true
  inviteCodeRegenerationCooldown: 2h
session:
  defaultReturnUrl: "http://localhost:4200/callback"
  accessValidity: 1h
  refreshValidity: 24h
  cookies:
    name: "session"
    domain: "localhost"
    path: "/"
    sameSite: Lax
    secure: false
  cors:
    allowedOrigins: ["localhost"]
    allowUnsafeWildcard: false
permify:
  url: <permify-host>:<permify-port>
argoController:
  kubeconfig: ""
  appProjectNamespace: "superphenix-system"
  garbageCollection:
    enabled: true
    interval: 15m
    timeout: 10m
    labelMarkKey: "superphenix.net/markedForDeletion"
    delay: 48h
    debug: false
auditLog:
  enabled: true
  retention:
    defaultDays: 90
    minDays: 1
    maxDays: 365
    userDays: 90
  garbageCollection:
    enabled: true
    interval: 1h
    timeout: 10m
    batchSize: 5000
database:
  host: ""
  port: ""
  username: ""
  password: ""
  database: ""
azs: {}
operator:
  kubeconfig: ""
  namespace: ""
spxPrefix: "spx"
argoCdUrl: "https://<argocd-host>"
productsConfig:
  argoApp:
    kubernetes:
      azDomains: {}
    backup:
      repo:
        repoURL: "oci://ghcr.io/super-phenix/charts/sfs-baas"
        chart: "sfs-baas"
        targetRevision: "0.2.1"
      schedule:
        minHour: 20
        maxHour: 23
  defaultVpc:
    productName: "default"
  defaultSubnet:
    productName: "default"
    protocol: "Dual"
    ipv4: "10.10.0.0/16"
    ipv6: "fd00:10:10::/64"
    natGatewayEnabled: false
    private: false
    dnsV4: "1.1.1.1"
    dnsV6: "2606:4700:4700::1111"
  snapshotSchedule:
    minHour: 21
    maxHour: 23
  s3:
    maxMinifiedJSONLen: 5000
`)

// Global is the global configuration of this application, provisioned once LoadConfig is called
var Global Config

func init() {
	if err := LoadConfig(); errors.Is(err, FileNotFound) {
		log.Print("Config file not found, falling back to environment variables and defaults")
	} else if err != nil {
		// An error was produced while loading the configuration
		log.Fatalf("An error occured while loading config file: %v", err)
	}
}

// LoadConfig loads the configuration from the file-system, environment variables and flags.
// The retrieved configuration is merged with the defaults values defined in this package,
// with user defined values taking priority over the hardcoded default values.
func LoadConfig() error {
	v := newViper()

	// Fetch configs from config.yaml
	v.SetConfigName("config")

	// Places where the config file can be stored
	v.AddConfigPath("/etc/" + AppName + "/")
	v.AddConfigPath(".")

	// Set the default configuration
	if err := loadDefaults(v); err != nil {
		return fmt.Errorf("failed to load default configuration: %s", err.Error())
	}

	// Find and read the config file supplied by the user
	err := v.ReadInConfig()
	if err != nil && errors.Is(err, err.(viper.ConfigFileNotFoundError)) {
		return FileNotFound
	}

	if err != nil {
		return fmt.Errorf("failed to read configuration file: %s", err.Error())
	}

	if err := v.Unmarshal(&Global); err != nil {
		return err
	}
	populateAZCodes()
	if err := Global.Validate(); err != nil {
		return err
	}
	return nil
}

// maxAuditLogBatchSize caps how many events one delete statement of the sweep removes.
const maxAuditLogBatchSize = 50000

// AuditLogConfig drives the audit log. Retention bounds the per-organization
// override; DefaultDays applies to organizations without one and to events
// attached to no organization.
type AuditLogConfig struct {
	Enabled   bool `yaml:"enabled"`
	Retention struct {
		DefaultDays int `yaml:"defaultDays"`
		MinDays     int `yaml:"minDays"`
		MaxDays     int `yaml:"maxDays"`
		// UserDays applies to the events attached to no organization. Platform-wide, not
		// overridable.
		UserDays int `yaml:"userDays"`
	} `yaml:"retention"`
	GarbageCollection struct {
		Enabled   bool          `yaml:"enabled"`
		Interval  time.Duration `yaml:"interval"`
		Timeout   time.Duration `yaml:"timeout"`
		BatchSize int           `yaml:"batchSize"`
	} `yaml:"garbageCollection"`
}

func (a *AuditLogConfig) validate() []string {
	if !a.Enabled {
		return nil
	}

	var errs []string
	retention := a.Retention
	if retention.MinDays < 1 || retention.MinDays > retention.DefaultDays || retention.DefaultDays > retention.MaxDays {
		errs = append(errs, "auditLog.retention must satisfy 1 <= minDays <= defaultDays <= maxDays")
	}
	if retention.UserDays < 1 {
		errs = append(errs, "auditLog.retention.userDays must be at least 1")
	}

	gc := a.GarbageCollection
	if !gc.Enabled {
		return errs
	}
	if gc.Interval <= 0 || gc.Timeout <= 0 {
		errs = append(errs, "auditLog.garbageCollection.interval and timeout must be positive")
	}
	if gc.BatchSize < 1 || gc.BatchSize > maxAuditLogBatchSize {
		errs = append(errs, fmt.Sprintf("auditLog.garbageCollection.batchSize must be between 1 and %d", maxAuditLogBatchSize))
	}
	return errs
}

// Validate enforces security-sensitive invariants on a loaded configuration.
// Called at the end of LoadConfig; a non-nil error must abort startup.
func (c *Config) Validate() error {
	var errs []string

	// Signing key must be strong: reject empty, the well-known default,
	// and anything shorter than 32 bytes (HS256 recommended minimum).
	secret := c.Authentication.JwtSecret
	switch {
	case secret == "":
		errs = append(errs, "authentication.jwtSecret must not be empty")
	case secret == "secret":
		errs = append(errs, `authentication.jwtSecret must not be the built-in default "secret"`)
	case len(secret) < 32:
		errs = append(errs, fmt.Sprintf("authentication.jwtSecret must be at least 32 bytes, got %d", len(secret)))
	}

	if c.Authentication.JwtIssuer == "" {
		errs = append(errs, "authentication.jwtIssuer must not be empty")
	}

	// AllowCredentials is true on the CORS middleware; combined with a "*"
	// origin this both violates the CORS spec in every major browser and
	// turns parseReturnUrl into an open redirect that leaks any token later
	// appended to the callback URL. Operators can opt in for local dev via
	// session.cors.allowUnsafeWildcard.
	hasWildcard := false
	for _, o := range c.Session.Cors.AllowedOrigins {
		if o == "*" {
			hasWildcard = true
			break
		}
	}
	if hasWildcard && !c.Session.Cors.AllowUnsafeWildcard {
		errs = append(errs, `session.cors.allowedOrigins contains "*"; set session.cors.allowUnsafeWildcard=true to enable it (dev only)`)
	}
	if hasWildcard && c.Session.Cors.AllowUnsafeWildcard {
		log.Print("WARNING: session.cors.allowedOrigins contains \"*\" and allowUnsafeWildcard is enabled — do not use this configuration in production")
	}

	errs = append(errs, c.AuditLog.validate()...)

	if c.Operator.Namespace == "" {
		errs = append(errs, "operator.namespace must not be empty")
	}
	errs = append(errs, c.ProductsConfig.ArgoApp.Kubernetes.validate()...)

	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// newViper returns a YAML viper instance using keyDelimiter, with values
// overridable by SUPERPHENIX-API_<PATH> env variables ("_" separated).
func newViper() *viper.Viper {
	v := viper.NewWithOptions(viper.KeyDelimiter(keyDelimiter))
	v.SetConfigType("yaml")
	v.SetEnvPrefix(strings.ToUpper(AppName))
	v.SetEnvKeyReplacer(strings.NewReplacer(keyDelimiter, "_"))
	v.AutomaticEnv()
	return v
}

// loadDefaults loads the default application configuration
func loadDefaults(v *viper.Viper) error {
	err := v.ReadConfig(bytes.NewBuffer(defaultConfig))
	if err != nil {
		return err
	}

	if err := v.Unmarshal(&Global); err != nil {
		return err
	}
	populateAZCodes()
	return nil
}

// populateAZCodes sets each AZConfig.Code from its map key.
func populateAZCodes() {
	for code, az := range Global.AZs {
		az.Code = code
		Global.AZs[code] = az
	}
}
