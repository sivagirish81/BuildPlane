package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/buildplane/buildplane/internal/api"
	"github.com/buildplane/buildplane/internal/config"
	"github.com/buildplane/buildplane/internal/database"
	"github.com/buildplane/buildplane/internal/kubernetes"
	"github.com/buildplane/buildplane/internal/queue"
	"github.com/buildplane/buildplane/internal/telemetry"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := config.Load("api")
	logger, shutdownTrace, _ := telemetry.Init("api")
	defer shutdownTrace(context.Background())

	store, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	q := queue.New(cfg.RedisAddr, cfg.RedisPassword)
	defer q.Close()
	kube, err := kubernetes.New(cfg)
	if err != nil {
		logger.Warn("kubernetes client unavailable; cancellation of active jobs disabled", "error", err)
	}
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.New(cfg, store, q, kube, logger).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		logger.Info("api listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}
