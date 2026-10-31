package dbaas

// DBaaSConfig is the AZ configuration served by the controller dbaas-config endpoint.
// ExternalSubnet is the subnet the EIPs of the databases are picked from.
type DBaaSConfig struct {
	StorageClasses []ClassMapping `json:"storageClasses"`
	ExternalSubnet string         `json:"externalSubnet"`
}

type ClassMapping struct {
	Shortname string `json:"name"`
	Fullname  string `json:"fullname"`
}

// DBaaSSpec is the user input of a database. Memory and Storage are in GiB.
// SubnetEId, VIP and StorageClass cannot change after creation.
type DBaaSSpec struct {
	Version        string `json:"version"`
	Instances      int    `json:"instances"`
	Cpu            int    `json:"cpu"`
	Memory         int    `json:"memory"`
	Storage        int    `json:"storage"`
	StorageClass   string `json:"storageClass"`
	SubnetEId      string `json:"subnetEId"`
	VIP            string `json:"vip"`
	ExternalAccess bool   `json:"externalAccess"`
}

// Subnet is the subnet the proxies of a database attach to.
type Subnet struct {
	LocalID string
	// CIDR is comma-separated when dual stack.
	CIDR string
}

type Values struct {
	Databases map[string]Database `yaml:"databases,omitempty"`
}

type Database struct {
	Name             string       `yaml:"name"`
	Location         string       `yaml:"location"`
	Version          string       `yaml:"version"`
	Instances        int          `yaml:"instances"`
	Cpu              string       `yaml:"cpu"`
	Memory           string       `yaml:"memory"`
	Storage          string       `yaml:"storage"`
	StorageClassName string       `yaml:"storageClassName"`
	Subnet           string       `yaml:"subnet"`
	SubnetCIDR       string       `yaml:"subnetCIDR"`
	LoadBalancer     LoadBalancer `yaml:"loadBalancer"`
	ExternalAccess   bool         `yaml:"externalAccess,omitempty"`
}

type LoadBalancer struct {
	VIP string `yaml:"vip"`
}
