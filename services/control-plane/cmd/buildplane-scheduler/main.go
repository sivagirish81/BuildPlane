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

	"github.com/sivagirish/buildplane/services/control-plane/internal/observability"
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

	redisQueue := queue.NewRedisQueue(redisClient)
	if err := redisQueue.EnsureConsumerGroup(startupCtx); err != nil {
		return err
	}

	repository := postgres.NewWorkflowRepository(db)
	scheduler := workflows.NewScheduler(repository, redisQueue)
	scheduler.BatchSize = envInt("BUILDPLANE_SCHEDULER_BATCH_SIZE", 10)
	metrics := observability.NewRegistry("buildplane-scheduler")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metricsErrCh := observability.StartMetricsServer(ctx, envString("BUILDPLANE_METRICS_ADDR", ":9090"), metrics, logger)

	interval := envDuration("BUILDPLANE_SCHEDULER_INTERVAL", 2*time.Second)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info("starting scheduler", "interval", interval.String(), "batch_size", scheduler.BatchSize)
	for {
		start := time.Now()
		published, seen, err := scheduler.RunOnce(ctx)
		status := "ok"
		if err != nil {
			status = "error"
			logger.Error("scheduler tick failed", "error", err)
		} else {
			logger.Info("scheduler tick completed", "published", published, "seen", seen)
		}
		metrics.Inc("buildplane_scheduler_ticks_total", map[string]string{"status": status})
		metrics.Add("buildplane_scheduler_tick_duration_seconds_sum", map[string]string{"status": status}, time.Since(start).Seconds())
		metrics.Inc("buildplane_scheduler_tick_duration_seconds_count", map[string]string{"status": status})
		metrics.Add("buildplane_scheduler_outbox_events_seen_total", nil, float64(seen))
		metrics.Add("buildplane_scheduler_outbox_events_published_total", nil, float64(published))

		select {
		case <-ctx.Done():
			logger.Info("shutting down scheduler")
			return nil
		case err := <-metricsErrCh:
			if err != nil {
				return err
			}
		case <-ticker.C:
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
