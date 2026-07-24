package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

const (
	ProductTypeInstance     = "instance"
	ProductTypeVPC          = "vpc"
	ProductTypeSubnet       = "subnet"
	ProductTypeEIP          = "eip"
	ProductTypeDisk         = "disk"
	ProductTypeSnapshot     = "snapshot"
	ProductTypeSSH          = "ssh"
	ProductTypeVmSnapshot   = "vmSnapshot"
	ProductTypeLoadBalancer = "loadBalancer"
	ProductTypeFirewall     = "firewall"
	ProductTypeKaaS         = "kaas"
	ProductTypeBaaS         = "baas"
	ProductTypeBucket       = "bucket"
)

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
	); err != nil {
		log.Error().Msg("Failed to auto migrate")
		return err
	}
	return nil
}
