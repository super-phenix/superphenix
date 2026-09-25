package authentication

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	auditEvent "github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/audit-event"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/audit"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type auditStore struct {
	inserted []model.AuditEvent
}

func (s *auditStore) Insert(_ context.Context, event model.AuditEvent) (uuid.UUID, error) {
	s.inserted = append(s.inserted, event)
	return uuid.New(), nil
}

func (s *auditStore) Finalize(context.Context, uuid.UUID, auditEvent.Completion) error { return nil }

func TestAuthenticateBeginsAuditEvent(t *testing.T) {
	userId := uuid.New()

	tests := []struct {
		name       string
		validation func(w http.ResponseWriter, r *http.Request) (*http.Request, error)
		wantEvents int
	}{
		{
			name: "validated user begins the event",
			validation: func(_ http.ResponseWriter, r *http.Request) (*http.Request, error) {
				return r.WithContext(context.WithValue(r.Context(), consts.ContextUserId, userId.String())), nil
			},
			wantEvents: 1,
		},
		{
			name: "failed validation leaves no event",
			validation: func(_ http.ResponseWriter, r *http.Request) (*http.Request, error) {
				return r, errors.New("validation failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &auditStore{}
			authType := AuthType{
				Name:       "TestAuth",
				Detection:  func(http.ResponseWriter, *http.Request) bool { return true },
				Validation: tt.validation,
			}

			audited := audit.Middleware(store)(router.Audit{Resource: router.Resource{Name: "disk"}, Action: router.ActionCreate})
			handler := audited(Authenticate(authType)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))

			if assert.Len(t, store.inserted, tt.wantEvents) && tt.wantEvents > 0 {
				assert.Equal(t, &userId, store.inserted[0].UserId)
				assert.Equal(t, "TestAuth", *store.inserted[0].AuthType)
				assert.Equal(t, model.AuditStatusAttempted, store.inserted[0].Status)
			}
		})
	}
}
