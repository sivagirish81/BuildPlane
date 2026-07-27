package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	workerID := os.Getenv("BUILDPLANE_WORKER_ID")
	if workerID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return fmt.Errorf("read hostname for worker id: %w", err)
		}
		workerID = hostname
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()

	db, err := postgres.Open(startupCtx, databaseURL)
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

	redisQueue := queue.NewRedisQueue(redisClient)
	if err := redisQueue.EnsureConsumerGroup(startupCtx); err != nil {
		return err
	}

	repository := postgres.NewWorkflowRepository(db)
	worker := workflows.NewWorker(repository, redisQueue, workerID)
	worker.LeaseDuration = envDuration("BUILDPLANE_WORKER_LEASE_DURATION", 30*time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("starting worker", "worker_id", workerID, "lease_duration", worker.LeaseDuration.String())
	for {
		processed, err := worker.RunOnce(ctx)
		if err != nil {
			logger.Error("worker iteration failed", "error", err, "worker_id", workerID)
		} else if processed {
			logger.Info("worker processed node execution", "worker_id", workerID)
		}

		select {
		case <-ctx.Done():
			logger.Info("shutting down worker", "worker_id", workerID)
			return nil
		default:
		}
	}
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
