package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/sivagirish/buildplane/services/control-plane/internal/postgres"
	"github.com/sivagirish/buildplane/services/control-plane/internal/queue"
	"github.com/sivagirish/buildplane/services/control-plane/internal/workflows"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("scheduler stopped", "error", err)
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
	scheduler := workflows.NewScheduler(repository, redisQueue)
	scheduler.BatchSize = envInt("BUILDPLANE_SCHEDULER_BATCH_SIZE", 10)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	interval := envDuration("BUILDPLANE_SCHEDULER_INTERVAL", 2*time.Second)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info("starting scheduler", "interval", interval.String(), "batch_size", scheduler.BatchSize)
	for {
		published, seen, err := scheduler.RunOnce(ctx)
		if err != nil {
			logger.Error("scheduler tick failed", "error", err)
		} else {
			logger.Info("scheduler tick completed", "published", published, "seen", seen)
		}

		select {
		case <-ctx.Done():
			logger.Info("shutting down scheduler")
			return nil
		case <-ticker.C:
		}
	}
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
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
