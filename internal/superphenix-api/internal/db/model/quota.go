package model

import (
	"github.com/google/uuid"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/quota"
)

// QuotaResources is the abstract quota of a level, e.g. {"cpu": "16", "memory": "64Gi", "disk": "1Ti"}.
type QuotaResources = quota.Resources

// The quota models are returned as is by the API: Resources, the limits set at the level, is
// exposed as `limit`, and the tracing timestamps are not exposed.
//
// Revision is the monotonic revision of the quota, read from the quota_revisions_seq sequence
// on every write (see the default below, which GORM leaves the database apply as long as the
// field is zero). It is sent to the AZ controllers, which refuse a push older than what they
// already enforce; it is never set by hand.

type OrganizationQuota struct {
	OrganizationId uuid.UUID      `gorm:"primaryKey;type:uuid;not null" json:"organizationId"`
	TracingModel   `json:"-"`
	Revision       int64          `gorm:"not null;default:nextval('quota_revisions_seq')" json:"-"`
	Resources      QuotaResources `gorm:"serializer:json;not null" json:"limit"`
}

func (OrganizationQuota) TableName() string { return "organization_quotas" }

type ProjectQuota struct {
	ProjectId      uuid.UUID      `gorm:"primaryKey;type:uuid;not null" json:"projectId"`
	OrganizationId uuid.UUID      `gorm:"type:uuid;not null;index" json:"organizationId"`
	TracingModel   `json:"-"`
	Revision       int64          `gorm:"not null;default:nextval('quota_revisions_seq')" json:"-"`
	Resources      QuotaResources `gorm:"serializer:json;not null" json:"limit"`
}

func (ProjectQuota) TableName() string { return "project_quotas" }

type ProjectAZQuota struct {
	ID           uuid.UUID      `gorm:"primaryKey;type:uuid;default:gen_random_uuid();not null" json:"-"`
	ProjectId    uuid.UUID      `gorm:"type:uuid;not null;index:idx_proj_az,unique" json:"projectId"`
	CodeAZ       string         `gorm:"not null;index:idx_proj_az,unique" json:"codeAZ"`
	TracingModel `json:"-"`
	Revision     int64          `gorm:"not null;default:nextval('quota_revisions_seq')" json:"-"`
	Resources    QuotaResources `gorm:"serializer:json;not null" json:"limit"`
}

func (ProjectAZQuota) TableName() string { return "project_az_quotas" }
