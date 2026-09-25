package model

import (
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"

	pwPermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Product types as audited resources; Name is the product_types key.
var (
	ProductTypeInstance      = router.Resource{Name: "instance", Label: "Instance"}
	ProductTypeVmSnapshot    = router.Resource{Name: "vmSnapshot", Label: "Instance Snapshot"}
	ProductTypeDisk          = router.Resource{Name: "disk", Label: "Disk"}
	ProductTypeSnapshot      = router.Resource{Name: "snapshot", Label: "Snapshot"}
	ProductTypeBaaS          = router.Resource{Name: "baas", Label: "Backup"}
	ProductTypeBucket        = router.Resource{Name: "bucket", Label: "Object Storage"}
	ProductTypeVPC           = router.Resource{Name: "vpc", Label: "VPC"}
	ProductTypeSubnet        = router.Resource{Name: "subnet", Label: "Subnet"}
	ProductTypeEIP           = router.Resource{Name: "eip", Label: "Elastic IP"}
	ProductTypeLoadBalancer  = router.Resource{Name: "loadBalancer", Label: "Load Balancer"}
	ProductTypeSecurityGroup = router.Resource{Name: "securityGroup", Label: "Security Group"}
	ProductTypeKaaS          = router.Resource{Name: "kaas", Label: "Kubernetes"}
	ProductTypeSSH           = router.Resource{Name: "ssh", Label: "SSH Keys"}
)

// ProductTypeReadPermission maps each product type to the permission that gates reading it.
var ProductTypeReadPermission = map[string]string{
	ProductTypeInstance.Name:      pwPermission.ProjectInstanceRead,
	ProductTypeVPC.Name:           pwPermission.ProjectVPCRead,
	ProductTypeSubnet.Name:        pwPermission.ProjectSubnetRead,
	ProductTypeEIP.Name:           pwPermission.ProjectEipRead,
	ProductTypeDisk.Name:          pwPermission.ProjectDiskRead,
	ProductTypeSnapshot.Name:      pwPermission.ProjectSnapshotRead,
	ProductTypeSSH.Name:           pwPermission.ProjectSSHRead,
	ProductTypeVmSnapshot.Name:    pwPermission.ProjectSnapshotRead,
	ProductTypeLoadBalancer.Name:  pwPermission.ProjectLoadBalancerRead,
	ProductTypeSecurityGroup.Name: pwPermission.ProjectSecurityGroupRead,
	ProductTypeKaaS.Name:          pwPermission.ProjectKaaSRead,
	ProductTypeBaaS.Name:          pwPermission.ProjectBaaSRead,
	ProductTypeBucket.Name:        pwPermission.ProjectBucketRead,
}

type Model struct {
	ID        uuid.UUID `gorm:"primaryKey;type:uuid;default:gen_random_uuid();not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
type TracingModel struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type User struct {
	Model
	Firstname *string
	Lastname  *string
	Email     string

	Provider   string
	ProviderId string

	InviteCode              uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();not null"`
	InviteCodeRegeneratedAt *time.Time `gorm:"default:null"`
	IsActive                bool       `gorm:"not null;default:false"` // Default value defined by config

	PersonalOrg []Organization  `gorm:"foreignKey:OwnerId"`
	GuestOrg    []*Organization `gorm:"many2many:user_organizations;"`
}

type UserOrganization struct {
	UserId         uuid.UUID `gorm:"primaryKey;type:uuid;"`
	OrganizationId uuid.UUID `gorm:"primaryKey;type:uuid;"`
	GroupId        uuid.UUID `gorm:"primaryKey;type:uuid;"`
}

type Organization struct {
	Model
	Name string

	OwnerId   uuid.UUID           `gorm:"type:uuid;"`
	Users     []*User             `gorm:"many2many:user_organizations;"`
	UserRoles []*UserOrganization `gorm:"foreignKey:OrganizationId"`
	Projects  []*Project          `gorm:"foreignKey:OrgaId"`

	AdministrativeContact *string
	BillingContact        *string
	TechnicalContact      *string

	// PredefinedCatalogVersion is the catalog version this organization is reconciled against.
	PredefinedCatalogVersion int `gorm:"not null;default:0"`

	// AuditRetentionDays overrides the configured audit log retention. Nil means the default.
	AuditRetentionDays *int
}

type Project struct {
	Model
	Name   string
	OrgaId uuid.UUID `gorm:"type:uuid;"`
}

type Group struct {
	Model
	Name string `gorm:"not null;"`

	OrgaId uuid.UUID `gorm:"type:uuid; not null"`
	// If true, then ignore ProjectIds
	AllProjects bool
	// List all projects concerned by the Group
	ProjectIds     []string `gorm:"serializer:json"`
	PermissionSets []string `gorm:"serializer:json"`

	// PredefinedKey links the group to an entry of v1.PredefinedGroups. Nil means the group is
	// custom: user-owned and never touched by the reconciler.
	PredefinedKey *string `gorm:"index"`
}

type ProductType struct {
	ID string `gorm:"primaryKey;not null"`
	TracingModel
	Name string `gorm:"not null"`
}

type Product struct {
	Model
	EffectiveID   string
	ProductName   string      `gorm:"not null"`
	CodeAZ        string      `gorm:"not null"`
	ProjectId     uuid.UUID   `gorm:"not null;type:uuid"`
	Project       Project     `gorm:"foreignKey:ProjectId"`
	ProductTypeId string      `gorm:"not null"`
	ProductType   ProductType `gorm:"foreignKey:ProductTypeId"`
}

type Quota struct {
	ID string `gorm:"primaryKey;not null"`
	TracingModel
	Value string `gorm:"not null"`
}

type QuotaOverride struct {
	ID string `gorm:"primaryKey;not null;"`
	TracingModel
	EntityId   uuid.UUID `gorm:"type:uuid;not null"`
	EntityType string    `gorm:"not null"`
	Value      string    `gorm:"not null"`
}

type ApiToken struct {
	Model

	Name string `gorm:"not null"`

	Prefix string `gorm:"not null"`

	UserId uuid.UUID `gorm:"type:uuid;not null"`
	User   User      `gorm:"foreignKey:UserId"`

	ExpiresAt time.Time

	Salt string `gorm:"not null"`

	TokenEncrypted string `gorm:"index;not null"`
}

const (
	AuditStatusAttempted = "attempted"
	AuditStatusSuccess   = "success"
	AuditStatusFailed    = "failed"
)

// AuditEvent is one audited action. Rows are append-only and hard deleted by the retention
// sweep. OrganizationId and ProjectId have no foreign key.
type AuditEvent struct {
	ID uuid.UUID `gorm:"primaryKey;type:uuid;default:gen_random_uuid();not null;index:idx_audit_events_org_started,priority:3,sort:desc"`

	OrganizationId *uuid.UUID `gorm:"type:uuid;index:idx_audit_events_org_started,priority:1"`
	ProjectId      *uuid.UUID `gorm:"type:uuid"`

	EventType    string `gorm:"not null"`
	ResourceType string `gorm:"not null"`
	ResourceId   *string

	UserId    *uuid.UUID `gorm:"type:uuid;index:idx_audit_events_user_started,priority:1,where:organization_id IS NULL"`
	UserEmail *string
	AuthType  *string

	// SourceIp is the client address taken from the proxy headers, RemoteAddr the raw peer.
	SourceIp   string
	RemoteAddr string

	Status     string `gorm:"not null"`
	StatusCode *int
	RequestId  string

	StartedAt   time.Time `gorm:"not null;index:idx_audit_events_org_started,priority:2,sort:desc;index:idx_audit_events_started;index:idx_audit_events_user_started,priority:2,sort:desc"`
	CompletedAt *time.Time
}

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&User{},
		&Organization{},
		&UserOrganization{},
		&Project{},
		&Quota{},
		&QuotaOverride{},
		&Group{},
		&ProductType{},
		&Product{},
		&ApiToken{},
		&AuditEvent{},
	); err != nil {
		log.Error().Msg("Failed to auto migrate")
		return err
	}
	return nil
}
