package baas

type BackupType string

type Values struct {
	Backups map[string]Backup `yaml:"backups,omitempty"`
}

type Backup struct {
	// User-friendly name of the backup
	Name string `yaml:"name"`
	// AZ in which to deploy the backup
	Location string `yaml:"location"`
	// Whether the backup is a single-shot or a schedule
	Scheduled bool `yaml:"scheduled"`
	// Define Interval for backups, only applied if ".scheduled" is true
	Schedule string `yaml:"schedule"`
	// Whether the backup schedule is paused, only applied if ".scheduled" is true
	Paused bool `yaml:"paused"`
	// Retention settings of the backups, only applied if ".scheduled" is true
	Retention RetentionPolicy `yaml:"retention"`
	// Label selector to target specific resources
	LabelSelector map[string]string `yaml:"labelSelector,omitempty"`
	// What will be backed up by this policy.
	// Available values: "All", "VM"
	Type BackupType `yaml:"type"`
}

type RetentionPolicy struct {
	//Retention time of the backups in hours (max is 960, or 40 days)
	ExpiryTime int `json:"expiryTime,omitempty" yaml:"expiryTime,omitempty"`
}

type BaaSSpec struct {
	Scheduled bool `json:"scheduled,omitempty"`
	// Schedule represent backup interval in days
	Schedule int `json:"schedule,omitempty"`
	// Retention is used only for Scheduled BaaS
	Retention     RetentionPolicy `json:"retention,omitempty"`
	Paused        bool            `json:"paused,omitempty"`
	LabelSelector []string        `json:"labelSelector,omitempty"`
	Type          BackupType      `json:"type,omitempty"`
}
