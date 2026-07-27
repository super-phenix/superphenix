package middleware

import (
	"net/http"
	"time"

	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	rslog "github.com/rs/zerolog/log"
)

// RequestLogger logs every incoming REST call with method, path, status,
// duration, and the logged-in user (automatically added by GetLogger).
// It uses Warn level for 4xx and Error level for 5xx status codes.
// Health check requests are logged minimally (method, path, status, duration).
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the ResponseWriter to capture the status code
		ww := chiMiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			status := ww.Status()

			// Health check: log minimally (like chi's default logger)
			if r.URL.Path == "/health" {
				rslog.Info().
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Int("status", status).
					Dur("duration", time.Since(start)).
					Msg("health check")
				return
			}

			log := logger.GetLogger(r.Context())

			// Resolve the matched route pattern (e.g. /{orgId}/{projectId}/instance/{effectiveId})
			routePattern := ""
			if rctx := chi.RouteContext(r.Context()); rctx != nil {
				routePattern = rctx.RoutePattern()
			}

			event := log.Info()
			if status >= 500 {
				event = log.Error()
			} else if status >= 400 {
				event = log.Warn()
			}

			event.
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("route", routePattern).
				Str("remoteAddr", r.RemoteAddr).
				Int("status", status).
				Int("bytes", ww.BytesWritten()).
				Dur("duration", time.Since(start)).
				Msg("REST call")
		}()

		next.ServeHTTP(ww, r)
	})
}
