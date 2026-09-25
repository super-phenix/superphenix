package auditlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	auditEvent "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/audit-event"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/utils"

	ch "github.com/super-phenix/superphenix/pkg/chi-helper"
	"github.com/super-phenix/superphenix/pkg/utils/decoder"
	httpError "github.com/super-phenix/superphenix/pkg/utils/error"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	defaultLimit = 25
	maxLimit     = 100
	// maxFilterLength bounds the free-text filters.
	maxFilterLength = 255
	// maxEventTypes bounds the repeatable eventType filter.
	maxEventTypes = 50
)

// Event is one audit event as returned by the API.
type Event struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationId *uuid.UUID `json:"organizationId,omitempty"`
	ProjectId      *uuid.UUID `json:"projectId,omitempty"`
	EventType      string     `json:"eventType"`
	ResourceType   string     `json:"resourceType"`
	ResourceId     *string    `json:"resourceId,omitempty"`
	UserId         *uuid.UUID `json:"userId,omitempty"`
	UserEmail      *string    `json:"userEmail,omitempty"`
	AuthType       *string    `json:"authType,omitempty"`
	// SourceIp is the client address taken from the proxy headers, falling back to the peer address.
	SourceIp    string     `json:"sourceIp"`
	Status      string     `json:"status" enums:"attempted,success,failed"`
	StatusCode  *int       `json:"statusCode,omitempty"`
	RequestId   string     `json:"requestId"`
	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// ListResponse is one page of events. Total counts every event matching the filters.
type ListResponse struct {
	Items  []Event `json:"items"`
	Total  int64   `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}

// Retention describes the audit log retention of an organization, in days.
type Retention struct {
	RetentionDays int  `json:"retentionDays"`
	IsDefault     bool `json:"isDefault"`
	DefaultDays   int  `json:"defaultDays"`
	MinDays       int  `json:"minDays"`
	MaxDays       int  `json:"maxDays"`
}

// UserRetention is the retention of the events attached to no organization, in days. It is set
// platform-wide and cannot be changed by users.
type UserRetention struct {
	RetentionDays int `json:"retentionDays"`
}

// UpdateRetentionBody sets the retention. A null retentionDays goes back to the default.
type UpdateRetentionBody struct {
	RetentionDays *int `json:"retentionDays"`
}

func toEvent(event model.AuditEvent) Event {
	return Event{
		ID:             event.ID,
		OrganizationId: event.OrganizationId,
		ProjectId:      event.ProjectId,
		EventType:      event.EventType,
		ResourceType:   event.ResourceType,
		ResourceId:     event.ResourceId,
		UserId:         event.UserId,
		UserEmail:      event.UserEmail,
		AuthType:       event.AuthType,
		SourceIp:       event.SourceIp,
		Status:         event.Status,
		StatusCode:     event.StatusCode,
		RequestId:      event.RequestId,
		StartedAt:      event.StartedAt,
		CompletedAt:    event.CompletedAt,
	}
}

func queryUUID(r *http.Request, key string) (*uuid.UUID, error) {
	value := ch.Query(r, key)
	if value == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be a UUID", key)
	}
	return &parsed, nil
}

func queryTime(r *http.Request, key string) (*time.Time, error) {
	value := ch.Query(r, key)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("%s must be an RFC 3339 date", key)
	}
	return &parsed, nil
}

func queryInt(r *http.Request, key string, fallback, lowest, highest int) (int, error) {
	value := ch.Query(r, key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < lowest || parsed > highest {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", key, lowest, highest)
	}
	return parsed, nil
}

func queryText(r *http.Request, key string) (string, error) {
	value := ch.Query(r, key)
	if len(value) > maxFilterLength {
		return "", fmt.Errorf("%s must be at most %d characters", key, maxFilterLength)
	}
	return value, nil
}

func validStatus(status string) bool {
	switch status {
	case "", model.AuditStatusAttempted, model.AuditStatusSuccess, model.AuditStatusFailed:
		return true
	}
	return false
}

// parseFilter reads the filters and the page from the query string.
func parseFilter(r *http.Request) (auditEvent.Filter, error) {
	var filter auditEvent.Filter
	var errs []error
	keep := func(err error) { errs = append(errs, err) }

	var err error
	filter.ProjectId, err = queryUUID(r, "projectId")
	keep(err)
	filter.UserId, err = queryUUID(r, "userId")
	keep(err)
	filter.From, err = queryTime(r, "from")
	keep(err)
	filter.To, err = queryTime(r, "to")
	keep(err)
	filter.UserEmail, err = queryText(r, "userEmail")
	keep(err)
	filter.ResourceType, err = queryText(r, "resourceType")
	keep(err)
	filter.ResourceId, err = queryText(r, "resourceId")
	keep(err)
	filter.Limit, err = queryInt(r, "limit", defaultLimit, 1, maxLimit)
	keep(err)
	filter.Offset, err = queryInt(r, "offset", 0, 0, int(^uint32(0)>>1))
	keep(err)

	filter.Status = ch.Query(r, "status")
	if !validStatus(filter.Status) {
		keep(errors.New("status must be attempted, success or failed"))
	}

	filter.EventTypes = ch.QueryArray(r, "eventType")
	if len(filter.EventTypes) > maxEventTypes {
		keep(fmt.Errorf("eventType accepts at most %d values", maxEventTypes))
	}
	for _, eventType := range filter.EventTypes {
		if len(eventType) > maxFilterLength {
			keep(fmt.Errorf("eventType must be at most %d characters", maxFilterLength))
			break
		}
	}

	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		keep(errors.New("from must not be after to"))
	}

	return filter, errors.Join(errs...)
}

func (s *Service) writeEvents(w http.ResponseWriter, r *http.Request, filter auditEvent.Filter, c *catalogue) {
	log := logger.GetLogger(r.Context())

	filter.EventTypes, filter.OtherThan = c.resolveOther(filter.EventTypes)
	events, total, err := s.store.List(r.Context(), filter)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list audit events")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to list audit events")
		return
	}

	response := ListResponse{Items: make([]Event, 0, len(events)), Total: total, Limit: filter.Limit, Offset: filter.Offset}
	for _, event := range events {
		response.Items = append(response.Items, toEvent(event))
	}
	writeJSON(w, r, response)
}

func writeJSON(w http.ResponseWriter, r *http.Request, payload any) {
	marshal, err := json.Marshal(payload)
	if err != nil {
		log := logger.GetLogger(r.Context())
		log.Error().Err(err).Msg("Error marshalling response")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to encode response")
		return
	}
	ch.Data(w, http.StatusOK, ch.MIMEJSON, marshal)
}

// ListOrganizationEvents
//
//	@Summary		List audit events
//	@Description	List the audit events of an organization, newest first. Events are read-only.
//	@Tags			v1, audit-log
//	@Produce		json
//	@Param			orgaId			path		string		true	"Organization ID"
//	@Param			eventType		query		[]string	false	"Event types, e.g. instance.create; `other` matches the types no longer declared"	collectionFormat(multi)
//	@Param			resourceType	query		string		false	"Resource type"
//	@Param			resourceId		query		string		false	"Resource ID"
//	@Param			userId			query		string		false	"Initiator user ID"
//	@Param			userEmail		query		string		false	"Initiator email"
//	@Param			projectId		query		string		false	"Project ID"
//	@Param			status			query		string		false	"Status"	Enums(attempted, success, failed)
//	@Param			from			query		string		false	"Started at or after, RFC 3339"
//	@Param			to				query		string		false	"Started at or before, RFC 3339"
//	@Param			limit			query		int			false	"Page size, 1 to 100"	default(25)
//	@Param			offset			query		int			false	"Events to skip"		default(0)
//	@Success		200				{object}	ListResponse
//	@Failure		400
//	@Failure		401
//	@Failure		403
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/audit-log [get]
//	@Security		Bearer[OrganizationRead, OrganizationAuditLogRead]
func (s *Service) ListOrganizationEvents(w http.ResponseWriter, r *http.Request) {
	orgaUuid, err := uuid.Parse(chi.URLParam(r, "orgaId"))
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("orgaId must be a UUID")
		return
	}

	filter, err := parseFilter(r)
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
		return
	}
	filter.OrganizationId = &orgaUuid

	s.writeEvents(w, r, filter, s.catalogue(true))
}

// ListUserEvents
//
//	@Summary		List my audit events outside organizations
//	@Description	List the caller's own audit events attached to no organization (API tokens, sessions).
//	@Tags			v1, audit-log
//	@Produce		json
//	@Param			eventType		query		[]string	false	"Event types, e.g. api-token.create; `other` matches the types no longer declared"	collectionFormat(multi)
//	@Param			resourceType	query		string		false	"Resource type"
//	@Param			resourceId		query		string		false	"Resource ID"
//	@Param			status			query		string		false	"Status"	Enums(attempted, success, failed)
//	@Param			from			query		string		false	"Started at or after, RFC 3339"
//	@Param			to				query		string		false	"Started at or before, RFC 3339"
//	@Param			limit			query		int			false	"Page size, 1 to 100"	default(25)
//	@Param			offset			query		int			false	"Events to skip"		default(0)
//	@Success		200				{object}	ListResponse
//	@Failure		400
//	@Failure		401
//	@Failure		500
//	@Router			/v1/user/audit-log [get]
//	@Security		Bearer
func (s *Service) ListUserEvents(w http.ResponseWriter, r *http.Request) {
	userUuid, err := utils.GetUserUuid(r)
	if err != nil {
		httpError.Http(w, r, http.StatusUnauthorized).Msg(http.StatusText(http.StatusUnauthorized))
		return
	}

	filter, err := parseFilter(r)
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg(err.Error())
		return
	}
	// Whatever the query says, a user only reads their own events.
	filter.UserId = &userUuid
	filter.UserEmail = ""
	filter.ProjectId = nil
	filter.NoOrganization = true

	s.writeEvents(w, r, filter, s.catalogue(false))
}

// ListOrganizationEventTypes
//
//	@Summary		List audit event types
//	@Description	List the event types that can appear in the audit log of an organization, with their resource label and action. Static per API version.
//	@Tags			v1, audit-log
//	@Produce		json
//	@Param			orgaId	path		string	true	"Organization ID"
//	@Success		200		{object}	EventTypesResponse
//	@Failure		400
//	@Failure		401
//	@Failure		403
//	@Router			/v1/organization/{orgaId}/audit-log/event-types [get]
//	@Security		Bearer[OrganizationRead, OrganizationAuditLogRead]
func (s *Service) ListOrganizationEventTypes(w http.ResponseWriter, r *http.Request) {
	if _, err := uuid.Parse(chi.URLParam(r, "orgaId")); err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("orgaId must be a UUID")
		return
	}
	writeJSON(w, r, EventTypesResponse{Items: s.catalogue(true).items})
}

// ListUserEventTypes
//
//	@Summary		List my audit event types
//	@Description	List the event types that can appear in the caller's own audit log, with their resource label and action. Static per API version.
//	@Tags			v1, audit-log
//	@Produce		json
//	@Success		200	{object}	EventTypesResponse
//	@Failure		401
//	@Router			/v1/user/audit-log/event-types [get]
//	@Security		Bearer
func (s *Service) ListUserEventTypes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, EventTypesResponse{Items: s.catalogue(false).items})
}

// GetUserRetention
//
//	@Summary		Get my audit log retention
//	@Description	Get how long the caller's audit events outside organizations are kept. Platform-wide, read-only.
//	@Tags			v1, audit-log
//	@Produce		json
//	@Success		200	{object}	UserRetention
//	@Failure		401
//	@Router			/v1/user/audit-log/retention [get]
//	@Security		Bearer
func (s *Service) GetUserRetention(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, UserRetention{RetentionDays: s.cfg.AuditLog.Retention.UserDays})
}

func (s *Service) retention(override *int) Retention {
	bounds := s.cfg.AuditLog.Retention
	retention := Retention{
		RetentionDays: bounds.DefaultDays,
		IsDefault:     true,
		DefaultDays:   bounds.DefaultDays,
		MinDays:       bounds.MinDays,
		MaxDays:       bounds.MaxDays,
	}
	if override != nil {
		// Clamp to the current bounds.
		retention.RetentionDays = min(max(*override, bounds.MinDays), bounds.MaxDays)
		retention.IsDefault = false
	}
	return retention
}

// GetRetention
//
//	@Summary		Get audit log retention
//	@Description	Get how long the audit events of the organization are kept, with the allowed bounds.
//	@Tags			v1, audit-log
//	@Produce		json
//	@Param			orgaId	path		string	true	"Organization ID"
//	@Success		200		{object}	Retention
//	@Failure		400
//	@Failure		401
//	@Failure		403
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/audit-log/retention [get]
//	@Security		Bearer[OrganizationRead, OrganizationAuditLogRead]
func (s *Service) GetRetention(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())

	orgaUuid, err := uuid.Parse(chi.URLParam(r, "orgaId"))
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("orgaId must be a UUID")
		return
	}

	override, err := s.store.Retention(orgaUuid)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read audit log retention")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to read audit log retention")
		return
	}

	writeJSON(w, r, s.retention(override))
}

// UpdateRetention
//
//	@Summary		Set audit log retention
//	@Description	Set how long the audit events of the organization are kept. Events older than the new retention are deleted by the next sweep.
//	@Tags			v1, audit-log
//	@Accept			json
//	@Produce		json
//	@Param			orgaId	path		string				true	"Organization ID"
//	@Param			Body	body		UpdateRetentionBody	true	"Retention in days, null for the default"
//	@Success		200		{object}	Retention
//	@Failure		400
//	@Failure		401
//	@Failure		403
//	@Failure		500
//	@Router			/v1/organization/{orgaId}/audit-log/retention [post]
//	@Security		Bearer[OrganizationRead, OrganizationAuditLogWrite]
func (s *Service) UpdateRetention(w http.ResponseWriter, r *http.Request) {
	log := logger.GetLogger(r.Context())

	orgaUuid, err := uuid.Parse(chi.URLParam(r, "orgaId"))
	if err != nil {
		httpError.Http(w, r, http.StatusBadRequest).Msg("orgaId must be a UUID")
		return
	}

	var body UpdateRetentionBody
	if err := decoder.HandleHTTPJSON(w, r, &body, s.cfg.PublicHTTP.MaxBodySize); err != nil {
		return
	}

	bounds := s.cfg.AuditLog.Retention
	if body.RetentionDays != nil && (*body.RetentionDays < bounds.MinDays || *body.RetentionDays > bounds.MaxDays) {
		httpError.Http(w, r, http.StatusBadRequest).
			Msg(fmt.Sprintf("retentionDays must be between %d and %d", bounds.MinDays, bounds.MaxDays))
		return
	}

	if err := s.store.SetRetention(orgaUuid, body.RetentionDays); err != nil {
		log.Error().Err(err).Msg("Failed to save audit log retention")
		httpError.Http(w, r, http.StatusInternalServerError).Msg("Failed to save audit log retention")
		return
	}

	writeJSON(w, r, s.retention(body.RetentionDays))
}
