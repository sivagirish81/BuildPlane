package telemetry

import (
	"context"
	"log/slog"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"net/http"
)

var (
	APIRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "buildplane_api_requests_total",
		Help: "HTTP API requests.",
	}, []string{"route", "method", "code"})
	SubmissionRejections = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "buildplane_submission_rejections_total",
		Help: "Rejected workflow submissions.",
	}, []string{"reason"})
	QueueDepth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "buildplane_queue_depth",
		Help: "Ready queue depth.",
	}, []string{"priority"})
	QueueOldestAge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "buildplane_queue_oldest_age_seconds",
		Help: "Oldest queued job age.",
	}, []string{"priority"})
	QueueWait = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "buildplane_queue_wait_seconds",
		Help: "Time spent in queue.",
	}, []string{"priority"})
	SchedulerDecisionDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "buildplane_scheduler_decision_duration_seconds",
		Help: "Scheduler decision latency.",
	})
	JobsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "buildplane_jobs_total",
		Help: "Jobs by terminal status.",
	}, []string{"status"})
	JobDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "buildplane_job_duration_seconds",
		Help: "Job execution duration.",
	}, []string{"status"})
	JobRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "buildplane_job_retries_total",
		Help: "Job retries.",
	}, []string{"reason"})
	ActiveJobs = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "buildplane_active_jobs",
		Help: "Active jobs.",
	})
	LeaseExpirations = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "buildplane_lease_expirations_total",
		Help: "Expired leases detected.",
	})
	StaleUpdatesRejected = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "buildplane_stale_updates_rejected_total",
		Help: "Runner updates rejected as stale.",
	})
	WorkerHeartbeats = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "buildplane_worker_heartbeats_total",
		Help: "Accepted worker heartbeats.",
	})
	CacheOperations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "buildplane_cache_operations_total",
		Help: "Cache operations.",
	}, []string{"operation", "result"})
	CacheBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "buildplane_cache_bytes_total",
		Help: "Cache transfer bytes.",
	}, []string{"operation"})
	KubernetesAPIErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "buildplane_kubernetes_api_errors_total",
		Help: "Kubernetes API errors.",
	}, []string{"operation"})
	AutoscalerDesiredCapacity = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "buildplane_autoscaler_desired_capacity",
		Help: "Autoscaler desired capacity.",
	})
	ReconciliationActions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "buildplane_reconciliation_actions_total",
		Help: "Reconciliation actions.",
	}, []string{"action"})
)

func Init(service string) (*slog.Logger, func(context.Context) error, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})).With("service", service)
	prometheus.MustRegister(APIRequests, SubmissionRejections, QueueDepth, QueueOldestAge, QueueWait,
		SchedulerDecisionDuration, JobsTotal, JobDuration, JobRetries, ActiveJobs, LeaseExpirations,
		StaleUpdatesRejected, WorkerHeartbeats, CacheOperations, CacheBytes, KubernetesAPIErrors,
		AutoscalerDesiredCapacity, ReconciliationActions)

	exporter, err := otlptracehttp.New(context.Background())
	if err != nil {
		logger.Warn("otel exporter disabled", "error", err)
		return logger, func(context.Context) error { return nil }, nil
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(service))),
	)
	otel.SetTracerProvider(tp)
	return logger, tp.Shutdown, nil
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
