package config

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	AppName   = "superphenix-api" // Name of the application in lowercase, used to determine the configuration path
	ApiPrefix = "/api/spx-ctrl"
)

var (
	FileNotFound = errors.New("couldn't find configuration file")
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
	Code          string   `yaml:"-"`
	AuthSecret    string   `yaml:"authSecret"`
	Name          string   `yaml:"name"`
	LogoUrl       string   `yaml:"logoUrl"`
	ControllerUrl string   `yaml:"controllerUrl"`
	Destination   string   `yaml:"destination"`
	Whitelist     []string `yaml:"whitelist"`
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

	Database struct {
		Host     string
		Port     string
		Username string
		Password string
		Database string
	}

	AZs map[string]AZConfig `yaml:"azs"`

	SpxPrefix string `yaml:"spxPrefix"`
	ArgoCdUrl string `yaml:"argoCdUrl"`

	ProductsConfig struct {
		ArgoApp struct {
			Kubernetes struct {
				Repo         RepoArgoAppConfig   `yaml:"repo"`
				KubeVersions []KubeVersionConfig `yaml:"kubeVersions,omitempty"`
				// AZ code -> internal/external URLs, sent in KaaS helm values.
				// The key must match the AZ code, which is what the chart's
				// `location` value is set to.
				AzDomains map[string]AzDomainConfig `yaml:"azDomains"`
			} `yaml:"kubernetes"`

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
database:
  host: ""
  port: ""
  username: ""
  password: ""
  database: ""
azs: {}
spxPrefix: "spx"
argoCdUrl: "https://<argocd-host>"
productsConfig:
  argoApp:
    kubernetes:
      repo:
        repoURL: "ghcr.io/super-phenix/charts"
        chart: "sfs-kaas"
        targetRevision: "0.6.3"
      kubeVersions:
        - version: "v1.36.3"
        - version: "v1.35.5"
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
	// Fetch configs from config.yaml
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Places where the config file can be stored
	viper.AddConfigPath("/etc/" + AppName + "/")
	viper.AddConfigPath(".")

	// Enable overriding values using env variables
	viper.SetEnvPrefix(strings.ToUpper(AppName))
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Set the default configuration
	if err := loadDefaults(); err != nil {
		return fmt.Errorf("failed to load default configuration: %s", err.Error())
	}

	// Find and read the config file supplied by the user
	err := viper.ReadInConfig()
	if err != nil && errors.Is(err, err.(viper.ConfigFileNotFoundError)) {
		return FileNotFound
	}

	if err != nil {
		return fmt.Errorf("failed to read configuration file: %s", err.Error())
	}

	if err := viper.Unmarshal(&Global); err != nil {
		return err
	}
	populateAZCodes()
	if err := Global.Validate(); err != nil {
		return err
	}
	return nil
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

	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// loadDefaults loads the default application configuration
func loadDefaults() error {
	err := viper.ReadConfig(bytes.NewBuffer(defaultConfig))
	if err != nil {
		return err
	}

	if err := viper.Unmarshal(&Global); err != nil {
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
