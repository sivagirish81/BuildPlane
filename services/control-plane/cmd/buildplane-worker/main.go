package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sivagirish/buildplane/services/control-plane/internal/observability"
	"github.com/sivagirish/buildplane/services/control-plane/internal/postgres"
	"github.com/sivagirish/buildplane/services/control-plane/internal/queue"
	"github.com/sivagirish/buildplane/services/control-plane/internal/workflows"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	databaseURL := os.Getenv("BUILDPLANE_DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("BUILDPLANE_DATABASE_URL is required")
	}
	redisURL := os.Getenv("BUILDPLANE_REDIS_URL")
	if redisURL == "" {
		return fmt.Errorf("BUILDPLANE_REDIS_URL is required")
	}
	workerPool := os.Getenv("BUILDPLANE_WORKER_POOL")
	if workerPool == "" {
		workerPool = workflows.WorkerPoolGeneral
	}

	workerID := os.Getenv("BUILDPLANE_WORKER_ID")
	if workerID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return fmt.Errorf("read hostname for worker id: %w", err)
		}
		workerID = fmt.Sprintf("%s-%s", workerPool, hostname)
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelStartup()

	db, err := postgres.OpenWithRetry(startupCtx, databaseURL, time.Second)
	if err != nil {
		return err
	}
	defer db.Close()

	migrationsDir := os.Getenv("BUILDPLANE_MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}
	if err := postgres.Migrate(startupCtx, db, migrationsDir); err != nil {
		return err
	}

	redisClient, err := queue.OpenRedis(redisURL)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	redisQueue := queue.NewRedisQueueForPool(redisClient, workerPool)
	if err := redisQueue.EnsureConsumerGroup(startupCtx); err != nil {
		return err
	}

	repository := postgres.NewWorkflowRepository(db)
	worker := workflows.NewWorker(repository, redisQueue, workerID)
	worker.LeaseDuration = envDuration("BUILDPLANE_WORKER_LEASE_DURATION", 30*time.Second)
	metrics := observability.NewRegistry("buildplane-worker")
	worker.Metrics = metrics
	drainTimeout := envDuration("BUILDPLANE_WORKER_DRAIN_TIMEOUT", 25*time.Second)
	aiServiceURL := os.Getenv("BUILDPLANE_AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://localhost:8090"
	}
	aiClient, err := workflows.NewHTTPAIClient(aiServiceURL, envDuration("BUILDPLANE_AI_SERVICE_TIMEOUT", 5*time.Second))
	if err != nil {
		return err
	}
	aiClient.SetMetrics(metrics)
	worker.Dependencies = workflows.NodeDependencies{
		AIClassifier: aiClient,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	metricsErrCh := observability.StartMetricsServer(ctx, envString("BUILDPLANE_METRICS_ADDR", ":9090"), metrics, logger)

	logger.Info(
		"starting worker",
		"worker_id", workerID,
		"worker_pool", workerPool,
		"lease_duration", worker.LeaseDuration.String(),
		"drain_timeout", drainTimeout.String(),
		"ai_service_url", aiServiceURL,
	)
	for {
		select {
		case <-ctx.Done():
			logger.Info("shutting down worker", "worker_id", workerID, "worker_pool", workerPool)
			return nil
		default:
		}

		iterationCtx, cancelIteration := context.WithTimeout(context.Background(), drainTimeout)
		processed, err := worker.RunOnce(iterationCtx)
		cancelIteration()
		if err != nil {
			logger.Error("worker iteration failed", "error", err, "worker_id", workerID, "worker_pool", workerPool)
		} else if processed {
			logger.Info("worker processed node execution", "worker_id", workerID, "worker_pool", workerPool)
		}

		select {
		case err := <-metricsErrCh:
			if err != nil {
				return err
			}
		default:
		}
	}
}

func envString(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
