// Package audit records the audited routes of the API as events.
//
// The middleware built by Middleware wraps a route and drops a Record in the request context.
// Authentication calls Begin once the user is known, which inserts the event as attempted.
// Handlers report what the URL does not carry with SetResource and SetProject. When the route
// returns, the middleware writes the outcome. A request that never authenticates leaves no event.
package audit

import (
	"context"
	"sync"
	"time"

	auditEvent "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/audit-event"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/metrics"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/google/uuid"
)

// Store persists events. The default one writes to the database.
type Store interface {
	Insert(ctx context.Context, event model.AuditEvent) (uuid.UUID, error)
	Finalize(ctx context.Context, id uuid.UUID, completion auditEvent.Completion) error
}

// DBStore is the Store backed by the audit_events table.
type DBStore struct{}

func (DBStore) Insert(ctx context.Context, event model.AuditEvent) (uuid.UUID, error) {
	return auditEvent.Insert(ctx, event)
}

func (DBStore) Finalize(ctx context.Context, id uuid.UUID, completion auditEvent.Completion) error {
	return auditEvent.Finalize(ctx, id, completion)
}

type recordKey struct{}

// Record is the event of the request in flight. Inner middlewares and the handler fill it
// through the package functions, the audit middleware reads it back once the route returns.
type Record struct {
	mu    sync.Mutex
	store Store

	event model.AuditEvent
	id    uuid.UUID
	// begun is true once the attempted row is stored.
	begun bool
	// failed is set by Fail for a refusal the status code does not show.
	failed bool
}

func fromContext(ctx context.Context) *Record {
	record, _ := ctx.Value(recordKey{}).(*Record)
	return record
}

// Begin stores the event as attempted on behalf of the authenticated user. It does nothing
// outside an audited route. A failed write is logged and never blocks the request.
func Begin(ctx context.Context, userId, authType string) {
	record := fromContext(ctx)
	if record == nil {
		return
	}

	parsed, err := uuid.Parse(userId)
	if err != nil {
		return
	}

	record.mu.Lock()
	defer record.mu.Unlock()
	if record.begun {
		return
	}

	record.event.UserId = &parsed
	record.event.AuthType = &authType
	record.event.Status = model.AuditStatusAttempted

	id, err := record.store.Insert(ctx, record.event)
	if err != nil {
		writeFailed(ctx, err)
		return
	}
	record.id = id
	record.begun = true
}

// SetResource reports the ID of the target resource when the URL does not carry it.
func SetResource(ctx context.Context, resourceId string) {
	if record := fromContext(ctx); record != nil && resourceId != "" {
		record.mu.Lock()
		record.event.ResourceId = &resourceId
		record.mu.Unlock()
	}
}

// ClearResource drops the resource reported by SetResource, for a creation that was rolled back.
func ClearResource(ctx context.Context) {
	if record := fromContext(ctx); record != nil {
		record.mu.Lock()
		record.event.ResourceId = nil
		record.mu.Unlock()
	}
}

// SetOrganization attaches the event to an organization the URL does not carry, such as the
// one a request just created.
func SetOrganization(ctx context.Context, organizationId uuid.UUID) {
	if record := fromContext(ctx); record != nil && organizationId != uuid.Nil {
		record.mu.Lock()
		record.event.OrganizationId = &organizationId
		record.mu.Unlock()
	}
}

// SetProject reports the project of the action when the URL does not carry it.
func SetProject(ctx context.Context, projectId uuid.UUID) {
	if record := fromContext(ctx); record != nil && projectId != uuid.Nil {
		record.mu.Lock()
		record.event.ProjectId = &projectId
		record.mu.Unlock()
	}
}

// Fail records the action as failed whatever the status code, for a refusal answered with a
// redirect.
func Fail(ctx context.Context) {
	if record := fromContext(ctx); record != nil {
		record.mu.Lock()
		record.failed = true
		record.mu.Unlock()
	}
}

// complete writes the outcome. Without a user there is nothing to write. When the attempted row
// could not be stored, the event is inserted in its final state instead.
func (record *Record) complete(ctx context.Context, statusCode int) {
	record.mu.Lock()
	defer record.mu.Unlock()
	if record.event.UserId == nil {
		return
	}

	status := model.AuditStatusSuccess
	if statusCode >= 400 || record.failed {
		status = model.AuditStatusFailed
	}
	now := time.Now()

	var err error
	if record.begun {
		err = record.store.Finalize(ctx, record.id, auditEvent.Completion{
			Status:         status,
			StatusCode:     statusCode,
			CompletedAt:    now,
			OrganizationId: record.event.OrganizationId,
			ProjectId:      record.event.ProjectId,
			ResourceId:     record.event.ResourceId,
		})
	} else {
		record.event.Status = status
		record.event.StatusCode = &statusCode
		record.event.CompletedAt = &now
		_, err = record.store.Insert(ctx, record.event)
	}
	if err != nil {
		writeFailed(ctx, err)
	}
}

func writeFailed(ctx context.Context, err error) {
	metrics.AuditWriteErrors.Inc()
	log := logger.GetLogger(ctx)
	log.Error().Err(err).Msg("Failed to write audit event")
}
