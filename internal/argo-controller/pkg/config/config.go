package config

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	AppName = "argo-controller" // Name of the application in lowercase, used to determine the configuration path
)

var (
	FileNotFound = errors.New("couldn't find configuration file")
)

type Config struct {
	Logging struct {
		Pretty bool
	}

	Permify struct {
		Url string `yaml:"url"`
	} `yaml:"permify"`

	Http struct {
		Address        string `yaml:"address"`
		HealthEndpoint string `yaml:"healthEndpoint"`
		MaxBodySize    uint   `yaml:"maxBodySize"`
		AuthSecret     string `yaml:"authSecret"`
	} `yaml:"http"`

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

	Swagger struct {
		BaseURL string `yaml:"baseUrl"`
	} `yaml:"swagger"`

	SpxPrefix string `yaml:"spxPrefix"`

	AppProjectNamespace string `yaml:"appProjectNamespace"`

	OrganizationWhitelist []string `yaml:"organizationWhitelist"`

	SuperAdmins []string `yaml:"superAdmins"`

	GarbageCollection struct {
		Interval     time.Duration `yaml:"interval"`
		Timeout      time.Duration `yaml:"timeout"`
		LabelMarkKey string        `yaml:"labelMarkKey"`
		Delay        time.Duration `yaml:"delay"`
		Debug        bool          `yaml:"debug"`
	} `yaml:"garbage_collection"`
}

var defaultConfig = []byte(`
logging:
  pretty: true
permify:
  url: "<permify-host>:<permify-port>"
http:
  address: ":8080"
  healthEndpoint: "/health"
  maxBodySize: 5
  authSecret: "secret"
superAdmins: []
tracing:
  enabled: false
  address: "<tracing-url>"
  cleanupTimeout: "5s"
  batchTimeout: "10s"
metrics:
  enabled: false
  endpoint: "/metrics"
  address: ""
  port: 9090
swagger:
  baseURL: "localhost:8080"
spxPrefix: "spx"
appProjectNamespace: "self-service-argocd"
organizationWhitelist: []
garbageCollection:
  interval: 15m
  timeout: 10m
  labelMarkKey: "superphenix.net/markedForDeletion"
  delay: 48h
  debug: true
`)

// Global is the global configuration of this application, provisioned once LoadConfig is called
var Global Config

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
