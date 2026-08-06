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
	AppName = "superphenix-controller" // Name of the application in lowercase, used to determine the configuration path
)

var (
	FileNotFound = errors.New("couldn't find configuration file")
)

// ContainerDiskCatalogEntry is one mountable container-disk type for this AZ.
// The ID is also used as the volume name on the VM spec.
type ContainerDiskCatalogEntry struct {
	ID          string   `yaml:"id" json:"id"`
	DisplayName string   `yaml:"displayName" json:"displayName"`
	Image       string   `yaml:"image" json:"image"`
	Bus         string   `yaml:"bus" json:"bus"` // "sata" | "virtio"
	SupportedOS []string `yaml:"supportedOS" json:"supportedOS"`
	Recommended bool     `yaml:"recommended" json:"recommended"`
}

type Config struct {
	AzName  string `yaml:"azName"`
	Logging struct {
		Pretty bool
	}

	ContainerDiskCatalog []ContainerDiskCatalogEntry `yaml:"containerDiskCatalog"`

	Http struct {
		Address    string `yaml:"address"`
		AuthSecret string `yaml:"authSecret"`
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

	ProductsConfig struct {
		NatGatewayDefault struct {
			ExternalSubnets []string `yaml:"externalSubnets"`
			DefaultRoutes   []struct {
				Cidr      string `yaml:"cidr"`
				NextHopIP string `yaml:"nextHopIP"`
			} `yaml:"defaultRoutes"`
			BgpSpeaker struct {
				Enabled               bool          `yaml:"enabled"`
				ASN                   uint32        `yaml:"asn"`
				RemoteASN             uint32        `yaml:"remoteAsn"`
				Neighbors             []string      `yaml:"neighbors"`
				HoldTime              time.Duration `yaml:"holdTime"`
				RouterID              string        `yaml:"routerId"`
				Password              string        `yaml:"password"`
				EnableGracefulRestart bool          `yaml:"enableGracefulRestart"`
				ExtraArgs             []string      `yaml:"extraArgs"`
			} `yaml:"bgpSpeaker"`
		} `yaml:"natGatewayDefault"`
		EipDefault struct {
			ExternalSubnet string `yaml:"externalSubnet"`
		} `yaml:"eipDefault"`

		Datavolume struct {
			DefaultAnnotations map[string]string `yaml:"defaultAnnotations"`
		} `yaml:"datavolume"`

		BlockStorage struct {
			StorageClassMapping map[string]string `yaml:"storageClassMapping"`
		} `yaml:"blockStorage"`

		ObjectStorage struct {
			StorageClassMapping map[string]string `yaml:"storageClassMapping"`
			MaxBucketSize       string            `yaml:"maxBucketSize"`
			MaxBucketObjects    uint64            `yaml:"maxBucketObjects"`
			ExternalEndpoint    string            `yaml:"externalEndpoint"`
		} `yaml:"objectStorage"`
	} `yaml:"productsConfig"`

	DisableEditionForResourcesByLabels map[string]string `yaml:"disableEditionForResourcesByLabels"`

	GarbageCollection struct {
		Interval     time.Duration `yaml:"interval"`
		Timeout      time.Duration `yaml:"timeout"`
		LabelMarkKey string        `yaml:"labelMarkKey"`
		Delay        time.Duration `yaml:"delay"`
		Debug        bool          `yaml:"debug"`
	} `yaml:"garbage_collection"`

	KubernetesConfig struct {
		QPS   float32 `yaml:"qps"`
		Burst int     `yaml:"burst"`
	} `yaml:"kubernetesConfig"`
}

var defaultConfig = []byte(`
azName: ""
logging:
  pretty: true
http:
  address: ":8080"
  authSecret: "secret"
tracing:
  enabled: false
  address: "http://94.23.250.59:14268/api/traces"
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
containerDiskCatalog: []
productsConfig:
  natGatewayDefault:
    externalSubnets:
      - spx-internal-bgp
    defaultRoutes:
      - cidr: 198.18.0.0/16
        nextHopIP: gateway
    bgpSpeaker:
      enabled: false
      asn: 65500
      remoteAsn: 65000
      neighbors:
        - "172.17.0.1"
        - "fd00:0:0:ffff::1"
      extraArgs:
        - -v5
        - --graceful-restart
  eipDefault:
    externalSubnet: external-subnet
  datavolume:
    defaultAnnotations:
      "v1.multus-cni.io/default-network": "kube-system/system-isolated-egress"
      "cdi.kubevirt.io/allowClaimAdoption": "true"
  blockStorage:
    storageClassMapping: {}
  objectStorage:
    storageClassMapping: {}
    maxBucketSize: "1Ti"
    maxBucketObjects: 1000000
    externalEndpoint: ""
disableEditionForResourcesByLabels:
  "app.kubernetes.io/name": "sfs-kaas"
garbageCollection:
  interval: 15m
  timeout: 10m
  labelMarkKey: "superphenix.net/markedForDeletion"
  delay: 48h
  debug: true
kubernetesConfig:
  qps: 100
  burst: 100
`)

// Global is the global configuration of this application, provisioned once LoadConfig is called
var Global Config

// v is a viper instance with a custom key delimiter to avoid treating dots
// in YAML map keys (e.g. "cdi.kubevirt.io/allowClaimAdoption") as nested paths.
var v = viper.NewWithOptions(viper.KeyDelimiter("::"))

// LoadConfig loads the configuration from the file-system, environment variables and flags.
// The retrieved configuration is merged with the defaults values defined in this package,
// with user defined values taking priority over the hardcoded default values.
func LoadConfig() error {
	// Fetch configs from config.yaml
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	// Places where the config file can be stored
	v.AddConfigPath("/etc/" + AppName + "/")
	v.AddConfigPath(".")

	// Enable overriding values using env variables
	v.SetEnvPrefix(strings.ToUpper(AppName))
	v.SetEnvKeyReplacer(strings.NewReplacer("::", "_"))
	v.AutomaticEnv()

	// Set the default configuration
	if err := loadDefaults(); err != nil {
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

	return v.Unmarshal(&Global)
}

// loadDefaults loads the default application configuration
func loadDefaults() error {
	err := v.ReadConfig(bytes.NewBuffer(defaultConfig))
	if err != nil {
		return err
	}

	return v.Unmarshal(&Global)
}
