package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "blog_api_http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestsDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "blog_api_http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	HTTPRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "blog_api_http_requests_in_flight",
			Help: "Current number of in-flight HTTP requests.",
		},
	)
)

// responseWriter wraps http.ResponseWriter to capture the status code
// written by the handler, since WriteHeader is called by the handler itself.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// MetricsMiddleware records RED metrics for the given fixed route path.
// path must be a fixed string (e.g. "/blog-api/posts/{slug}"), never r.URL.Path,
// to avoid unbounded label cardinality in Prometheus.
func MetricsMiddleware(path string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		HTTPRequestsInFlight.Inc()
		defer HTTPRequestsInFlight.Dec()

		start := time.Now()
		next.ServeHTTP(rw, r)
		duration := time.Since(start).Seconds()

		HTTPRequestsDuration.WithLabelValues(r.Method, path).Observe(duration)
		HTTPRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(rw.status)).Inc()
	})
}
