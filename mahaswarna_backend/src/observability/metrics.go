package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency by method, path, and status code.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
	}, []string{"service", "method", "path", "status"})

	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests by method, path, and status code.",
	}, []string{"service", "method", "path", "status"})

	WSConnectionsActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ws_connections_active",
		Help: "Active WebSocket connections.",
	}, []string{"service"})

	KafkaMessagesConsumed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_messages_consumed_total",
		Help: "Kafka messages consumed.",
	}, []string{"service", "topic", "status"})

	DBQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "db_query_duration_seconds",
		Help:    "PostgreSQL query latency.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5},
	}, []string{"service", "query"})
)

// MetricsHandler returns the Prometheus /metrics HTTP handler.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// InstrumentHandler wraps an http.Handler with duration and total-count metrics.
func InstrumentHandler(service, method, path string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, code: 200}
		next.ServeHTTP(rw, r)
		dur := time.Since(start).Seconds()
		code := strconv.Itoa(rw.code)
		HTTPRequestDuration.WithLabelValues(service, method, path, code).Observe(dur)
		HTTPRequestsTotal.WithLabelValues(service, method, path, code).Inc()
	})
}

type responseWriter struct {
	http.ResponseWriter
	code int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.code = code
	rw.ResponseWriter.WriteHeader(code)
}
