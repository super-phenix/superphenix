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
	AppName = "superphenix-api" // Name of the application in lowercase, used to determine the configuration path
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
	Code          string   `yaml:"code"`
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
		Enabled        bool
		HealthEndpoint string
		Address        string
		AuthSecret     string
	}

	BillingHTTP struct {
		AuthSecret string
	}

	SuperAdmins []string `yaml:"superAdmins"`

	Redis struct {
		Address  string
		Password string
		Database int
		Timeout  time.Duration
	}

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
		KratosCallback string
		KratosEndpoint string
		RedirectURL    string
	}

	Swagger struct {
		BaseURL string
	}
	Session struct {
		UserIsActiveOnCreate           bool          `json:"userIsActiveOnCreate"` // If false, need manual activation in database
		InviteCodeRegenerationCooldown time.Duration `yaml:"inviteCodeRegenerationCooldown"`

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
		}
	}

	Permify struct {
		Url string
	}

	Controller struct {
		ApiPrefix  string
		AuthSecret string
	}

	ArgoController struct {
		Url        string
		AuthSecret string

		App struct {
			KaaS struct {
				Repo             RepoArgoAppConfig   `yaml:"repo"`
				KubeVersions     []KubeVersionConfig `yaml:"kubeVersions,omitempty"`
				KubeConfigDomain string              `yaml:"kubeConfigDomain"`
				// AZ code -> domain, sent in KaaS helm values.
				AzDomains map[string]string `yaml:"azDomains"`
			}

			BaaS struct {
				Repo     RepoArgoAppConfig `yaml:"repo"`
				Schedule struct {
					MinHour int `yaml:"minHour"`
					MaxHour int `yaml:"maxHour"`
				} `yaml:"schedule"`
			}
		} `yaml:"app"`
	}

	Database struct {
		Host     string
		Port     string
		Username string
		Password string
		Database string
	}

	AZs []AZConfig `yaml:"azs"`

	S3 struct {
		// MaxMinifiedJSONLen caps bucket policy/lifecycle JSON minified length
		MaxMinifiedJSONLen int `yaml:"maxMinifiedJSONLen"`
	} `yaml:"s3"`

	SpxPrefix string `yaml:"spxPrefix"`
	ArgoCdUrl string `yaml:"argoCdUrl"`

	DefaultProducts struct {
		VPC struct {
			ProductName string `yaml:"productName"`
		} `yaml:"vpc"`
		Subnet struct {
			ProductName       string `yaml:"productName"`
			Protocol          string `yaml:"protocol"`
			IPv4              string `yaml:"ipv4"`
			IPv6              string `yaml:"ipv6"`
			NatGatewayEnabled bool   `yaml:"natGatewayEnabled"`
			Private           bool   `yaml:"private"`
		} `yaml:"subnet"`
	} `yaml:"defaultProducts"`
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
  healthEndpoint: "/healthz"
  address: ":7000"
  authSecret: ""
  azHealthTimeout: 15
billingHttp:
  authSecret: ""
superAdmins: []
redis:
  address: "<redis-host>:<redis-port>"
  password: "<redis-password>"
  database: 0
  timeout: 10s
tracing:
  enabled: true
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
  kratosCallback: "http://<kratos-callback-host>/v1/auth/link/%s/callback"
  kratosEndpoint: "https://<kratos-public-host>"
  redirectUrl: "https://<kratos-public-host>/self-service/login/browser?refresh=true&return_to=%s"
swagger:
  baseURL: "localhost:8080"
session:
  userIsActiveOnCreate: false
  inviteCodeRegenerationCooldown: 2h
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
    allowedOrigins: ["*"]
permify:
  url: <permify-host>:<permify-port>
controller:
  apiPrefix: "/api/spx-ctrl"
  authSecret: "secret"
argoController:
  url: "argo-url"
  authSecret: "secret"
  app:
    kaas:
      kubeConfigDomain: "<kube-config-domain>"
      kubeVersions: []
      azDomains: {}
    baas:
      schedule:
        minHour: 20
        maxHour: 23
database:
  host: ""
  port: ""
  username: ""
  password: ""
  database: ""
azs: []
s3:
  maxMinifiedJSONLen: 5000
spxPrefix: "spx"
argoCdUrl: "https://<argocd-host>"
defaultProducts:
  vpc:
    productName: "default"
  subnet:
    productName: "default"
    protocol: "Dual"
    ipv4: "10.10.0.0/16"
    ipv6: "fd00:10:10::/64"
    natGatewayEnabled: false 
    private: false
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

	return viper.Unmarshal(&Global)
}

// loadDefaults loads the default application configuration
func loadDefaults() error {
	err := viper.ReadConfig(bytes.NewBuffer(defaultConfig))
	if err != nil {
		return err
	}

	return viper.Unmarshal(&Global)
}
