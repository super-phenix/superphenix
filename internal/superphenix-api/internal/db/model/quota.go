package model

import (
	"github.com/google/uuid"
)

type QuotaResources struct {
	CPU    string `json:"cpu,omitempty"`    // e.g. "16", "32000m"
	Memory string `json:"memory,omitempty"` // e.g. "64Gi", "68719476736"
	Disk   string `json:"disk,omitempty"`   // e.g. "1Ti", "1099511627776"
}

type OrganizationQuota struct {
	OrganizationId uuid.UUID      `gorm:"primaryKey;type:uuid;not null"`
	TracingModel
	Resources      QuotaResources `gorm:"serializer:json;not null"`
}

func (OrganizationQuota) TableName() string { return "organization_quotas" }

type ProjectQuota struct {
	ProjectId      uuid.UUID      `gorm:"primaryKey;type:uuid;not null"`
	OrganizationId uuid.UUID      `gorm:"type:uuid;not null;index"`
	TracingModel
	Resources      QuotaResources `gorm:"serializer:json;not null"`
}

func (ProjectQuota) TableName() string { return "project_quotas" }

type ProjectAZQuota struct {
	ID             uuid.UUID      `gorm:"primaryKey;type:uuid;default:gen_random_uuid();not null"`
	ProjectId      uuid.UUID      `gorm:"type:uuid;not null;index:idx_proj_az,unique"`
	CodeAZ         string         `gorm:"not null;index:idx_proj_az,unique"`
	TracingModel
	Resources      QuotaResources `gorm:"serializer:json;not null"`
}

func (ProjectAZQuota) TableName() string { return "project_az_quotas" }
