package model

import (
	"time"

	"github.com/google/uuid"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/quota"
)

// QuotaResources is the abstract quota of a level, e.g. {"cpu": "16", "memory": "64Gi", "disk": "1Ti"}.
type QuotaResources = quota.Resources

// The quota models are returned as is by the API: Resources, the limits set at the level, is
// exposed as `limit`, and the tracing timestamps are not exposed.

type OrganizationQuota struct {
	OrganizationId uuid.UUID `gorm:"primaryKey;type:uuid;not null" json:"organizationId"`
	TracingModel   `json:"-"`
	Resources      QuotaResources `gorm:"serializer:json;not null" json:"limit"`
}

func (OrganizationQuota) TableName() string { return "organization_quotas" }

type ProjectQuota struct {
	ProjectId      uuid.UUID `gorm:"primaryKey;type:uuid;not null" json:"projectId"`
	OrganizationId uuid.UUID `gorm:"type:uuid;not null;index" json:"organizationId"`
	TracingModel   `json:"-"`
	Resources      QuotaResources `gorm:"serializer:json;not null" json:"limit"`
}

func (ProjectQuota) TableName() string { return "project_quotas" }

type ProjectAZQuota struct {
	ID           uuid.UUID `gorm:"primaryKey;type:uuid;default:gen_random_uuid();not null" json:"-"`
	ProjectId    uuid.UUID `gorm:"type:uuid;not null;index:idx_proj_az,unique" json:"projectId"`
	CodeAZ       string    `gorm:"not null;index:idx_proj_az,unique" json:"codeAZ"`
	TracingModel `json:"-"`
	Resources    QuotaResources `gorm:"serializer:json;not null" json:"limit"`
}

func (ProjectAZQuota) TableName() string { return "project_az_quotas" }

// ProjectAZQuotaState is the sync state of the quota of a project in one AZ, written by the quota
// writes (WantedRevision) and the quota reconciler (the other fields). Revisions are quota
// revisions of the organization.
type ProjectAZQuotaState struct {
	ProjectId      uuid.UUID `gorm:"primaryKey;type:uuid"`
	CodeAZ         string    `gorm:"primaryKey"`
	OrganizationId uuid.UUID `gorm:"type:uuid;not null;index"`
	// WantedRevision is the revision of the last write that changed the quota of the AZ.
	WantedRevision int64 `gorm:"not null;default:0"`
	// PreparedRevision is the revision of the last push prepared: only its outcome is recorded.
	PreparedRevision int64 `gorm:"not null;default:0"`
	// AppliedRevision is the revision of the last push the AZ applied.
	AppliedRevision int64 `gorm:"not null;default:0"`
	// Held is the AZ quota the AZ may enforce: the one it applied, raised by every push prepared
	// since. The AZ is allocated the highest of Held and its AZ quota.
	Held QuotaResources `gorm:"serializer:json"`
	// DueAt is when the reconciler takes the state next.
	DueAt time.Time `gorm:"not null;default:now();index"`
	// LeaseToken identifies the reconciler that took the state last.
	LeaseToken *uuid.UUID `gorm:"type:uuid"`
	// Attempts counts the failed pushes since the last one applied.
	Attempts int64 `gorm:"not null;default:0"`
	// LastError is the error of the last push, nil once a push is applied.
	LastError *string
}

func (ProjectAZQuotaState) TableName() string { return "project_az_quota_states" }
