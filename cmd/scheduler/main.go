package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/buildplane/buildplane/internal/config"
	"github.com/buildplane/buildplane/internal/database"
	"github.com/buildplane/buildplane/internal/kubernetes"
	"github.com/buildplane/buildplane/internal/queue"
	"github.com/buildplane/buildplane/internal/scheduler"
	"github.com/buildplane/buildplane/internal/telemetry"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := config.Load("scheduler")
	logger, shutdownTrace, _ := telemetry.Init("scheduler")
	defer shutdownTrace(context.Background())
	store, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	q := queue.New(cfg.RedisAddr, cfg.RedisPassword)
	defer q.Close()
	kube, err := kubernetes.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	logger.Info("scheduler starting")
	if err := scheduler.New(cfg, store, q, kube, logger).Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
