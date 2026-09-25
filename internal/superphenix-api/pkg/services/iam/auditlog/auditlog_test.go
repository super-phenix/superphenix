package auditlog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	auditEvent "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/audit-event"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type fakeStore struct {
	events   []model.AuditEvent
	total    int64
	override *int
	err      error

	gotFilter    auditEvent.Filter
	gotRetention *int
	setCalled    bool
}

func (s *fakeStore) List(_ context.Context, filter auditEvent.Filter) ([]model.AuditEvent, int64, error) {
	s.gotFilter = filter
	return s.events, s.total, s.err
}

func (s *fakeStore) Retention(uuid.UUID) (*int, error) { return s.override, s.err }

func (s *fakeStore) SetRetention(_ uuid.UUID, days *int) error {
	s.setCalled = true
	s.gotRetention = days
	return s.err
}

func testConfig() *config.Config {
	var cfg config.Config
	cfg.PublicHTTP.MaxBodySize = 1
	cfg.AuditLog.Retention.DefaultDays = 90
	cfg.AuditLog.Retention.MinDays = 7
	cfg.AuditLog.Retention.MaxDays = 365
	cfg.AuditLog.Retention.UserDays = 30
	return &cfg
}

// testDeclared covers both logs, a duplicate declaration, an undeclared read and a skipped write.
func testDeclared() []router.RouteInfo {
	disk := router.Resource{Name: "disk", Label: "Disk"}
	return []router.RouteInfo{
		{Method: http.MethodPost, Pattern: "/v1/organization/{orgaId}/disk", Audit: &router.Audit{Resource: disk, Action: router.ActionCreate}},
		{Method: http.MethodPut, Pattern: "/v1/organization/{orgaId}/disk/{id}", Audit: &router.Audit{Resource: disk, Action: router.ActionCreate}},
		{
			Method: http.MethodPost, Pattern: "/v1/organization",
			Audit: &router.Audit{Resource: router.Resource{Name: "organization", Label: "Organization"}, Action: router.ActionCreate, ReportsOrganization: true},
		},
		{Method: http.MethodPost, Pattern: "/v1/api-token", Audit: &router.Audit{Resource: router.Resource{Name: "api-token", Label: "API Token"}, Action: router.ActionCreate}},
		{Method: http.MethodGet, Pattern: "/v1/session/token", Audit: &router.Audit{Resource: router.Resource{Name: "session", Label: "Session"}, Action: "login"}},
		{Method: http.MethodGet, Pattern: "/v1/organization/{orgaId}"},
		{Method: http.MethodPost, Pattern: "/v1/organization/{orgaId}/search", Audit: &router.Audit{Skip: true, SkipReason: "read-only"}},
	}
}

func testService(store Store) *Service {
	return NewWithDeclared(testConfig(), store, testDeclared)
}

// serve routes the request through chi so the URL params resolve, as the given user.
func serve(s *Service, method, target, body string, userId *uuid.UUID) *httptest.ResponseRecorder {
	root := chi.NewRouter()
	root.Get("/v1/organization/{orgaId}/audit-log", s.ListOrganizationEvents)
	root.Get("/v1/organization/{orgaId}/audit-log/event-types", s.ListOrganizationEventTypes)
	root.Get("/v1/organization/{orgaId}/audit-log/retention", s.GetRetention)
	root.Post("/v1/organization/{orgaId}/audit-log/retention", s.UpdateRetention)
	root.Get("/v1/user/audit-log", s.ListUserEvents)
	root.Get("/v1/user/audit-log/event-types", s.ListUserEventTypes)
	root.Get("/v1/user/audit-log/retention", s.GetUserRetention)

	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if userId != nil {
		r = r.WithContext(context.WithValue(r.Context(), consts.ContextUserId, userId.String()))
	}
	rr := httptest.NewRecorder()
	root.ServeHTTP(rr, r)
	return rr
}

func TestListOrganizationEvents(t *testing.T) {
	orgId := uuid.New()
	projectId := uuid.New()
	base := "/v1/organization/" + orgId.String() + "/audit-log"

	tests := []struct {
		name       string
		target     string
		storeErr   error
		wantStatus int
		check      func(t *testing.T, filter auditEvent.Filter)
	}{
		{
			name:       "defaults",
			target:     base,
			wantStatus: http.StatusOK,
			check: func(t *testing.T, filter auditEvent.Filter) {
				assert.Equal(t, &orgId, filter.OrganizationId)
				assert.Equal(t, defaultLimit, filter.Limit)
				assert.Zero(t, filter.Offset)
				assert.False(t, filter.NoOrganization)
			},
		},
		{
			name: "every filter",
			target: base + "?eventType=disk.create&eventType=disk.delete&resourceType=disk&resourceId=spx-1" +
				"&userEmail=a%40b.c&projectId=" + projectId.String() +
				"&status=failed&from=2026-09-01T00:00:00Z&to=2026-09-02T00:00:00Z&limit=10&offset=20",
			wantStatus: http.StatusOK,
			check: func(t *testing.T, filter auditEvent.Filter) {
				assert.Equal(t, []string{"disk.create", "disk.delete"}, filter.EventTypes)
				assert.Equal(t, "disk", filter.ResourceType)
				assert.Equal(t, "spx-1", filter.ResourceId)
				assert.Equal(t, "a@b.c", filter.UserEmail)
				assert.Equal(t, &projectId, filter.ProjectId)
				assert.Equal(t, model.AuditStatusFailed, filter.Status)
				assert.NotNil(t, filter.From)
				assert.NotNil(t, filter.To)
				assert.Equal(t, 10, filter.Limit)
				assert.Equal(t, 20, filter.Offset)
			},
		},
		{
			name:       "other keeps the types no longer declared",
			target:     base + "?eventType=other",
			wantStatus: http.StatusOK,
			check: func(t *testing.T, filter auditEvent.Filter) {
				assert.Empty(t, filter.EventTypes)
				assert.Equal(t, []string{"disk.create", "organization.create"}, filter.OtherThan)
			},
		},
		{
			name:       "other next to a declared type",
			target:     base + "?eventType=disk.create&eventType=other",
			wantStatus: http.StatusOK,
			check: func(t *testing.T, filter auditEvent.Filter) {
				assert.Equal(t, []string{"disk.create"}, filter.EventTypes)
				assert.Equal(t, []string{"disk.create", "organization.create"}, filter.OtherThan)
			},
		},
		{
			name:       "no other, no exclusion",
			target:     base + "?eventType=disk.create",
			wantStatus: http.StatusOK,
			check: func(t *testing.T, filter auditEvent.Filter) {
				assert.Equal(t, []string{"disk.create"}, filter.EventTypes)
				assert.Nil(t, filter.OtherThan)
			},
		},
		{
			name:       "other counts against the limit",
			target:     base + "?" + strings.Repeat("eventType=disk.create&", maxEventTypes) + "eventType=other",
			wantStatus: http.StatusBadRequest,
		},
		{name: "organization is not a uuid", target: "/v1/organization/nope/audit-log", wantStatus: http.StatusBadRequest},
		{name: "unknown status", target: base + "?status=done", wantStatus: http.StatusBadRequest},
		{name: "limit above the maximum", target: base + "?limit=101", wantStatus: http.StatusBadRequest},
		{name: "limit of zero", target: base + "?limit=0", wantStatus: http.StatusBadRequest},
		{name: "negative offset", target: base + "?offset=-1", wantStatus: http.StatusBadRequest},
		{name: "malformed date", target: base + "?from=yesterday", wantStatus: http.StatusBadRequest},
		{name: "from after to", target: base + "?from=2026-09-02T00:00:00Z&to=2026-09-01T00:00:00Z", wantStatus: http.StatusBadRequest},
		{name: "malformed user id", target: base + "?userId=me", wantStatus: http.StatusBadRequest},
		{name: "oversized filter", target: base + "?resourceId=" + strings.Repeat("a", maxFilterLength+1), wantStatus: http.StatusBadRequest},
		{name: "store failure", target: base, storeErr: errors.New("boom"), wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{events: []model.AuditEvent{{EventType: "disk.create"}}, total: 42, err: tt.storeErr}
			rr := serve(testService(store), http.MethodGet, tt.target, "", nil)

			assert.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantStatus != http.StatusOK {
				return
			}

			var response ListResponse
			assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
			assert.Equal(t, int64(42), response.Total)
			assert.Len(t, response.Items, 1)
			assert.Equal(t, store.gotFilter.Limit, response.Limit)
			tt.check(t, store.gotFilter)
		})
	}
}

func TestListUserEvents(t *testing.T) {
	userId := uuid.New()
	other := uuid.New()

	tests := []struct {
		name       string
		target     string
		userId     *uuid.UUID
		wantStatus int
	}{
		{name: "own events only", target: "/v1/user/audit-log", userId: &userId, wantStatus: http.StatusOK},
		{
			name:       "cannot read another user or a project",
			target:     "/v1/user/audit-log?userId=" + other.String() + "&userEmail=x%40y.z&projectId=" + other.String(),
			userId:     &userId,
			wantStatus: http.StatusOK,
		},
		{name: "other resolves against the user log", target: "/v1/user/audit-log?eventType=other", userId: &userId, wantStatus: http.StatusOK},
		{name: "no user in context", target: "/v1/user/audit-log", wantStatus: http.StatusUnauthorized},
		{name: "invalid filter", target: "/v1/user/audit-log?limit=abc", userId: &userId, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{}
			rr := serve(testService(store), http.MethodGet, tt.target, "", tt.userId)

			assert.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantStatus != http.StatusOK {
				return
			}
			if strings.Contains(tt.target, "eventType=other") {
				assert.Equal(t, []string{"api-token.create", "session.login"}, store.gotFilter.OtherThan)
			}
			assert.Equal(t, &userId, store.gotFilter.UserId)
			assert.True(t, store.gotFilter.NoOrganization)
			assert.Nil(t, store.gotFilter.OrganizationId)
			assert.Nil(t, store.gotFilter.ProjectId)
			assert.Empty(t, store.gotFilter.UserEmail)
		})
	}
}

func TestRetention(t *testing.T) {
	base := "/v1/organization/" + uuid.NewString() + "/audit-log/retention"
	days := func(n int) *int { return &n }

	tests := []struct {
		name       string
		method     string
		body       string
		override   *int
		storeErr   error
		wantStatus int
		want       Retention
		wantSaved  *int
		wantSet    bool
	}{
		{
			name: "default when no override", method: http.MethodGet, wantStatus: http.StatusOK,
			want: Retention{RetentionDays: 90, IsDefault: true, DefaultDays: 90, MinDays: 7, MaxDays: 365},
		},
		{
			name: "override", method: http.MethodGet, override: days(30), wantStatus: http.StatusOK,
			want: Retention{RetentionDays: 30, DefaultDays: 90, MinDays: 7, MaxDays: 365},
		},
		{
			name: "override outside the current bounds is clamped", method: http.MethodGet, override: days(1), wantStatus: http.StatusOK,
			want: Retention{RetentionDays: 7, DefaultDays: 90, MinDays: 7, MaxDays: 365},
		},
		{name: "read failure", method: http.MethodGet, storeErr: errors.New("boom"), wantStatus: http.StatusInternalServerError},
		{
			name: "set", method: http.MethodPost, body: `{"retentionDays": 30}`, wantStatus: http.StatusOK,
			want:      Retention{RetentionDays: 30, DefaultDays: 90, MinDays: 7, MaxDays: 365},
			wantSaved: days(30), wantSet: true,
		},
		{
			name: "null goes back to the default", method: http.MethodPost, body: `{"retentionDays": null}`, wantStatus: http.StatusOK,
			want:    Retention{RetentionDays: 90, IsDefault: true, DefaultDays: 90, MinDays: 7, MaxDays: 365},
			wantSet: true,
		},
		{name: "below the minimum", method: http.MethodPost, body: `{"retentionDays": 6}`, wantStatus: http.StatusBadRequest},
		{name: "above the maximum", method: http.MethodPost, body: `{"retentionDays": 366}`, wantStatus: http.StatusBadRequest},
		{name: "malformed body", method: http.MethodPost, body: `{"retentionDays": "long"}`, wantStatus: http.StatusBadRequest},
		{
			name: "save failure", method: http.MethodPost, body: `{"retentionDays": 30}`, storeErr: errors.New("boom"),
			wantStatus: http.StatusInternalServerError, wantSaved: days(30), wantSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{override: tt.override, err: tt.storeErr}
			rr := serve(NewWithStore(testConfig(), store), tt.method, base, tt.body, nil)

			assert.Equal(t, tt.wantStatus, rr.Code)
			assert.Equal(t, tt.wantSet, store.setCalled)
			assert.Equal(t, tt.wantSaved, store.gotRetention)
			if tt.wantStatus != http.StatusOK {
				return
			}

			var got Retention
			assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetUserRetention(t *testing.T) {
	tests := []struct {
		name     string
		userDays int
		want     UserRetention
	}{
		{name: "reports the configured value", userDays: 30, want: UserRetention{RetentionDays: 30}},
		{name: "follows the config", userDays: 7, want: UserRetention{RetentionDays: 7}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.AuditLog.Retention.UserDays = tt.userDays
			rr := serve(NewWithStore(cfg, &fakeStore{}), http.MethodGet, "/v1/user/audit-log/retention", "", nil)

			assert.Equal(t, http.StatusOK, rr.Code)
			var got UserRetention
			assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestListEventTypes(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantStatus int
		want       []EventType
	}{
		{
			name:       "organization log, sorted by resource then action, deduplicated",
			target:     "/v1/organization/" + uuid.NewString() + "/audit-log/event-types",
			wantStatus: http.StatusOK,
			want: []EventType{
				{EventType: "disk.create", ResourceType: "disk", ResourceLabel: "Disk", Action: "create"},
				{EventType: "organization.create", ResourceType: "organization", ResourceLabel: "Organization", Action: "create"},
			},
		},
		{
			name:       "user log",
			target:     "/v1/user/audit-log/event-types",
			wantStatus: http.StatusOK,
			want: []EventType{
				{EventType: "api-token.create", ResourceType: "api-token", ResourceLabel: "API Token", Action: "create"},
				{EventType: "session.login", ResourceType: "session", ResourceLabel: "Session", Action: "login"},
			},
		},
		{name: "organization is not a uuid", target: "/v1/organization/nope/audit-log/event-types", wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := serve(testService(&fakeStore{}), http.MethodGet, tt.target, "", nil)

			assert.Equal(t, tt.wantStatus, rr.Code)
			if tt.wantStatus != http.StatusOK {
				return
			}
			var got EventTypesResponse
			assert.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
			assert.Equal(t, tt.want, got.Items)
		})
	}
}

func TestEventTypesWithoutDeclaration(t *testing.T) {
	rr := serve(NewWithStore(testConfig(), &fakeStore{}), http.MethodGet, "/v1/user/audit-log/event-types", "", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"items": []}`, rr.Body.String())
}
