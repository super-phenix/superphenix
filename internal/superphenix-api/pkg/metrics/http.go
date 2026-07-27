package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPLabels = []string{"method", "status"}

	HTTPProcessTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: Namespace,
		Name:      "http_process_time",
		Help:      "HTTP processing time in milliseconds",
		Buckets: []float64{
			0.1,
			0.125,
			0.25,
			0.50,
			0.75,
			1,
			5,
			10,
			50,
			100,
			200,
			300,
			400,
			500,
			600,
			700,
			800,
			900,
			1000,
		},
	})

	HTTPRequestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "http_request_count",
		Help:      "Requests received by status code and method",
	}, HTTPLabels)
)

// MiddlewareHTTP records metrics about the HTTP requests received by the API
// This middleware is very generic and only records processing time grouped by
// HTTP method and HTTP return status
func MiddlewareHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStartTime := time.Now()

		wrappedResponse := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(wrappedResponse, r)

		elapsedTime := time.Since(requestStartTime).Milliseconds()
		HTTPProcessTime.Observe(float64(elapsedTime))

		statusCode := strconv.Itoa(wrappedResponse.Status())
		HTTPRequestCount.WithLabelValues(r.Method, statusCode).Inc()
	})
}
