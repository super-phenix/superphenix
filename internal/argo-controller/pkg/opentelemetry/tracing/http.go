package tracing

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// MiddlewareHTTP is an HTTP middleware that handles creating traces for each HTTP request that passes through it
func MiddlewareHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, span := otel.Tracer("").Start(r.Context(), r.URL.Path)
		defer span.End()

		wrappedResponse := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(wrappedResponse, r)

		span.SetAttributes(
			attribute.String("remote.address", r.RemoteAddr),
			attribute.String("http.method", r.Method),
			attribute.String("http.uri", r.RequestURI),
			attribute.Int("http.status_code", wrappedResponse.Status()),
		)
	})
}
