package audit

import (
	"context"
	"net/http"
	"time"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/router"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

const (
	paramOrganization = "orgaId"
	paramProject      = "projectId"

	// completeTimeout bounds the final write, which runs detached from the request.
	completeTimeout = 5 * time.Second
)

// Middleware returns the factory handed to router.Registry.SetAuditor.
func Middleware(store Store) func(router.Audit) router.Middleware {
	return func(declaration router.Audit) router.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				record := newRecord(r, store, declaration)
				ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

				defer func() {
					// The client may be gone, the outcome is written anyway.
					ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), completeTimeout)
					defer cancel()

					if recovered := recover(); recovered != nil {
						record.complete(ctx, http.StatusInternalServerError)
						panic(recovered)
					}

					statusCode := ww.Status()
					if statusCode == 0 {
						statusCode = http.StatusOK
					}
					record.complete(ctx, statusCode)
				}()

				next.ServeHTTP(ww, r.WithContext(context.WithValue(r.Context(), recordKey{}, record)))
			})
		}
	}
}

func newRecord(r *http.Request, store Store, declaration router.Audit) *Record {
	record := &Record{
		store: store,
		event: model.AuditEvent{
			OrganizationId: uuidParam(r, paramOrganization),
			ProjectId:      uuidParam(r, paramProject),
			EventType:      declaration.EventType(),
			ResourceType:   declaration.Resource.Name,
			SourceIp:       ClientIP(r),
			RemoteAddr:     PeerAddr(r),
			RequestId:      middleware.GetReqID(r.Context()),
			StartedAt:      time.Now(),
		},
	}
	resourceId := ""
	switch {
	case declaration.ResourceParam != "":
		resourceId = chi.URLParam(r, declaration.ResourceParam)
	case declaration.ResourceQuery != "":
		resourceId = r.URL.Query().Get(declaration.ResourceQuery)
	}
	if resourceId != "" {
		record.event.ResourceId = &resourceId
	}
	return record
}

// uuidParam returns the URL param as a UUID, nil when absent or malformed.
func uuidParam(r *http.Request, name string) *uuid.UUID {
	parsed, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return nil
	}
	return &parsed
}
