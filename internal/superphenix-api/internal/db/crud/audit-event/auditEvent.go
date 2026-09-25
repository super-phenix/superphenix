// Package auditEvent persists audit events.
package auditEvent

import (
	"context"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const tableName = "audit_events"

// Filter narrows List. Zero values are ignored.
type Filter struct {
	OrganizationId *uuid.UUID
	// NoOrganization keeps only the events attached to no organization.
	NoOrganization bool
	ProjectId      *uuid.UUID
	UserId         *uuid.UUID
	UserEmail      string
	EventTypes     []string
	// OtherThan, when set, also matches the events whose type is not in it.
	OtherThan    []string
	ResourceType string
	ResourceId   string
	Status       string
	From         *time.Time
	To           *time.Time

	Limit  int
	Offset int
}

// Completion is the final state written once the audited request returns.
type Completion struct {
	Status      string
	StatusCode  int
	CompletedAt time.Time
	// OrganizationId, ProjectId and ResourceId are written when set.
	OrganizationId *uuid.UUID
	ProjectId      *uuid.UUID
	ResourceId     *string
}

// Insert stores the event and returns its ID. The user email is resolved in the same statement.
func Insert(ctx context.Context, event model.AuditEvent) (uuid.UUID, error) {
	id := uuid.New()

	var email any
	if event.UserId != nil {
		email = gorm.Expr("(SELECT email FROM users WHERE id = ?)", *event.UserId)
	}

	res := db.Client.WithContext(ctx).Table(tableName).Create(map[string]any{
		"id":              id,
		"organization_id": event.OrganizationId,
		"project_id":      event.ProjectId,
		"event_type":      event.EventType,
		"resource_type":   event.ResourceType,
		"resource_id":     event.ResourceId,
		"user_id":         event.UserId,
		"user_email":      email,
		"auth_type":       event.AuthType,
		"source_ip":       event.SourceIp,
		"remote_addr":     event.RemoteAddr,
		"status":          event.Status,
		"status_code":     event.StatusCode,
		"request_id":      event.RequestId,
		"started_at":      event.StartedAt,
		"completed_at":    event.CompletedAt,
	})

	return id, res.Error
}

// Finalize writes the outcome of an event inserted as attempted.
func Finalize(ctx context.Context, id uuid.UUID, completion Completion) error {
	values := map[string]any{
		"status":       completion.Status,
		"status_code":  completion.StatusCode,
		"completed_at": completion.CompletedAt,
	}
	if completion.OrganizationId != nil {
		values["organization_id"] = *completion.OrganizationId
	}
	if completion.ProjectId != nil {
		values["project_id"] = *completion.ProjectId
	}
	if completion.ResourceId != nil {
		values["resource_id"] = *completion.ResourceId
	}

	return db.Client.WithContext(ctx).Table(tableName).Where("id = ?", id).Updates(values).Error
}

func applyFilter(query *gorm.DB, filter Filter) *gorm.DB {
	if filter.OrganizationId != nil {
		query = query.Where("organization_id = ?", *filter.OrganizationId)
	}
	if filter.NoOrganization {
		query = query.Where("organization_id IS NULL")
	}
	if filter.ProjectId != nil {
		query = query.Where("project_id = ?", *filter.ProjectId)
	}
	if filter.UserId != nil {
		query = query.Where("user_id = ?", *filter.UserId)
	}
	if filter.UserEmail != "" {
		query = query.Where("lower(user_email) = lower(?)", filter.UserEmail)
	}
	switch {
	case len(filter.EventTypes) > 0 && len(filter.OtherThan) > 0:
		query = query.Where("event_type IN ? OR event_type NOT IN ?", filter.EventTypes, filter.OtherThan)
	case len(filter.EventTypes) > 0:
		query = query.Where("event_type IN ?", filter.EventTypes)
	case len(filter.OtherThan) > 0:
		query = query.Where("event_type NOT IN ?", filter.OtherThan)
	}
	if filter.ResourceType != "" {
		query = query.Where("resource_type = ?", filter.ResourceType)
	}
	if filter.ResourceId != "" {
		query = query.Where("resource_id = ?", filter.ResourceId)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.From != nil {
		query = query.Where("started_at >= ?", *filter.From)
	}
	if filter.To != nil {
		query = query.Where("started_at <= ?", *filter.To)
	}

	return query
}

// List returns one page of events, newest first, and the total matching the filter.
func List(ctx context.Context, filter Filter) ([]model.AuditEvent, int64, error) {
	var total int64
	count := applyFilter(db.Client.WithContext(ctx).Model(&model.AuditEvent{}), filter)
	if err := count.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	events := make([]model.AuditEvent, 0, filter.Limit)
	res := applyFilter(db.Client.WithContext(ctx).Model(&model.AuditEvent{}), filter).
		Order("started_at DESC, id DESC").
		Limit(filter.Limit).
		Offset(filter.Offset).
		Find(&events)

	return events, total, res.Error
}

// DeleteExpiredByOrganization hard deletes up to limit events of the organization started before
// cutoff and returns how many went.
func DeleteExpiredByOrganization(ctx context.Context, orgId uuid.UUID, cutoff time.Time, limit int) (int64, error) {
	res := db.Client.WithContext(ctx).Exec(`DELETE FROM audit_events WHERE id IN (
		SELECT id FROM audit_events
		WHERE organization_id = ? AND started_at < ?
		ORDER BY started_at LIMIT ?)`, orgId, cutoff, limit)

	return res.RowsAffected, res.Error
}

// DeleteExpiredWithoutOrganization does the same for the events attached to no organization.
func DeleteExpiredWithoutOrganization(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	res := db.Client.WithContext(ctx).Exec(`DELETE FROM audit_events WHERE id IN (
		SELECT id FROM audit_events
		WHERE organization_id IS NULL AND started_at < ?
		ORDER BY started_at LIMIT ?)`, cutoff, limit)

	return res.RowsAffected, res.Error
}

// DeleteExpiredDefault does the same for the organizations without a retention override.
func DeleteExpiredDefault(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	res := db.Client.WithContext(ctx).Exec(`DELETE FROM audit_events WHERE id IN (
		SELECT e.id FROM audit_events e
		WHERE e.organization_id IS NOT NULL AND e.started_at < ? AND NOT EXISTS (
			SELECT 1 FROM organizations o
			WHERE o.id = e.organization_id AND o.audit_retention_days IS NOT NULL)
		ORDER BY e.started_at LIMIT ?)`, cutoff, limit)

	return res.RowsAffected, res.Error
}
