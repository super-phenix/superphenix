package kaas

type GroupDeletion struct {
	GroupName []string `json:"groupName"`
}

type KaaSConfig struct {
	StorageClasses []ClassMapping `json:"storageClasses"`
}

type ClassMapping struct {
	Shortname string `json:"name"`
	Fullname  string `json:"fullname"`
}

type KaaSSpec struct {
	KubeVersion      string               `json:"kubeVersion"`
	CPNetPol         string               `json:"cpNetPol"`
	WorkersNetPol    string               `json:"workersNetPol"`
	Groups           []Group              `json:"groups"`
	KaasEssentials   EssentialsSpec       `json:"kaasEssentials"`
	PostInstallChart PostInstallChartSpec `json:"postInstallChart,omitempty"`
	ControlPlane     ControlPlaneSpec     `json:"controlPlane,omitempty"`
}

type ControlPlaneSpec struct {
	DataStore DataStoreSpec `json:"dataStore"`
}

// DataStoreSpec is the disaster recovery (PRA) input: a dedicated datastore for
// the control plane. Storage is in GiB, formatted to "<n>Gi" like BootDiskSize.
type DataStoreSpec struct {
	Dedicated        bool   `json:"dedicated"`
	StorageClassName string `json:"storageClassName,omitempty"`
	Storage          int    `json:"storage,omitempty"`
}

type Group struct {
	Name         string        `json:"name" validate:"max=63"`
	Replicas     int           `json:"replicas"`
	Cpu          int           `json:"cpu"`
	Memory       int           `json:"memory"`
	BootDiskSize int           `json:"bootDiskSize"`
	StorageClass string        `json:"storageClass"`
	Subnets      []GroupSubnet `json:"subnets"`
	Version      int           `json:"version,omitempty"`
}

type PostInstallChartSpec struct {
	ChartName    string `json:"chartName,omitempty"`
	ChartVersion string `json:"chartVersion,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
	RepoUrl      string `json:"repoUrl,omitempty"`
	Revision     int    `json:"revision,omitempty"`
	Values       string `json:"values,omitempty"`
}

type EssentialsSpec struct {
	Revision            int    `json:"revision,omitempty"`
	CorednsValues       string `json:"corednsValues,omitempty"`
	CiliumValues        string `json:"ciliumValues,omitempty"`
	MetricsServerValues string `json:"metricsServerValues,omitempty"`
}

type GroupSubnet struct {
	Order int    `json:"order"`
	Id    string `json:"id"` // Subnet localId

}

// AzDomain Internal is the host the in-VM kube-API proxy targets
// External a URL template for the control plane FQDN where "%s"
// is replaced by the cluster ID.
type AzDomain struct {
	Internal string `yaml:"internal,omitempty"`
	External string `yaml:"external,omitempty"`
}

type Values struct {
	AzDomains map[string]AzDomain `yaml:"azDomains,omitempty"`
	Clusters  map[string]Cluster  `yaml:"clusters,omitempty"`
}

type Cluster struct {
	Name             string           `yaml:"name,omitempty"`
	Location         string           `yaml:"location,omitempty"`
	KubeVersion      string           `yaml:"kubeVersion,omitempty"`
	ControlPlane     ControlPlane     `yaml:"controlPlane,omitempty"` // Auto compute by the helm chart
	Workers          Workers          `yaml:"workers,omitempty"`
	PostInstallChart PostInstallChart `yaml:"postInstallChart,omitempty"`
	KaaSEssentials   Essentials       `yaml:"kaasEssentials,omitempty"`
}

type ControlPlane struct {
	DataStore *DataStore `yaml:"dataStore,omitempty"`
	Paused    bool       `yaml:"paused,omitempty"`
	Network   struct {
		DefaultPolicies string `yaml:"defaultPolicies,omitempty"`
		Fqdn            string `yaml:"fqdn,omitempty"`
	} `yaml:"network,omitempty"`
}

type DataStore struct {
	Name             string `yaml:"name,omitempty"`
	Dedicated        bool   `yaml:"dedicated"`
	StorageClassName string `yaml:"storageClassName,omitempty"`
	Storage          string `yaml:"storage,omitempty"`
}

type Workers struct {
	Instances map[string]Instance `yaml:"instances,omitempty"`
	Network   struct {
		DefaultPolicies string `yaml:"defaultPolicies,omitempty"`
	} `yaml:"network,omitempty"`
}

type Interface struct {
	Subnet string `yaml:"subnet,omitempty"`
}

type Instance struct {
	KubeVersion string     `yaml:"kubeVersion,omitempty"`
	Deployment  Deployment `yaml:"deployment,omitempty"`
	Template    Template   `yaml:"template,omitempty"`
}

type Deployment struct {
	Replicas       int    `yaml:"replicas,omitempty"`
	NodeRole       string `yaml:"nodeRole,omitempty"`
	MaxSurge       int    `yaml:"maxSurge,omitempty"`
	MaxUnavailable int    `yaml:"maxUnavailable,omitempty"`
}

type Template struct {
	Version           int         `yaml:"version,omitempty"`
	Cores             int         `yaml:"cores,omitempty"`
	Memory            string      `yaml:"memory,omitempty"`
	BootDisk          BootDisk    `yaml:"bootDisk,omitempty"`
	ImageTag          string      `yaml:"imageTag,omitempty"`
	Interfaces        []Interface `yaml:"interfaces,omitempty"`
	AdditionalVolumes struct {
		Disks map[string]BootDisk `yaml:"disks,omitempty"`
	} `yaml:"additionalVolumes,omitempty"`
}

type BootDisk struct {
	Name             string `yaml:"name,omitempty"`
	Storage          string `yaml:"storage,omitempty"`
	StorageClassName string `yaml:"storageClassName,omitempty"`
	Source           struct {
		Registry struct {
			URL string `yaml:"url,omitempty"`
		} `yaml:"registry,omitempty"`
	} `yaml:"source,omitempty"`
}

type Essentials struct {
	ChartVersion    string                  `yaml:"chartVersion,omitempty"`
	Revision        int                     `yaml:"revision"`
	StorageClasses  map[string]StorageClass `yaml:"storageClasses,omitempty"`
	SnapshotClasses map[string]StorageClass `yaml:"snapshotClasses,omitempty"`
	Values          EssentialsValues        `yaml:"values,omitempty"`
}

type PostInstallChart struct {
	ChartName    string      `yaml:"chartName,omitempty"`
	ChartVersion string      `yaml:"chartVersion,omitempty"`
	Namespace    string      `yaml:"namespace,omitempty"`
	RepoUrl      string      `yaml:"repoUrl,omitempty"`
	Revision     int         `yaml:"revision,omitempty"`
	Values       interface{} `yaml:"values,omitempty"`
}

type EssentialsValues struct {
	Coredns       interface{} `yaml:"coredns,omitempty"`
	Cilium        interface{} `yaml:"cilium,omitempty"`
	MetricsServer interface{} `yaml:"metrics-server,omitempty"`
}

type StorageClass struct {
	IsDefaultClass bool   `yaml:"isDefaultClass,omitempty"`
	TenantClass    string `yaml:"tenantClass,omitempty"`
	InfraClass     string `yaml:"infraClass,omitempty"`
}
