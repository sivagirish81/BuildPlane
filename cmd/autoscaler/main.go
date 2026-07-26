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

	"github.com/buildplane/buildplane/internal/autoscaling"
	"github.com/buildplane/buildplane/internal/config"
	"github.com/buildplane/buildplane/internal/database"
	"github.com/buildplane/buildplane/internal/telemetry"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := config.Load("autoscaler")
	logger, shutdownTrace, _ := telemetry.Init("autoscaler")
	defer shutdownTrace(context.Background())
	store, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", telemetry.MetricsHandler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		current := cfg.GlobalActiveLimit
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				active, _, cpu, _, err := store.ActiveCounts(ctx)
				if err != nil {
					logger.Warn("autoscaler active count failed", "error", err)
					continue
				}
				queued, _ := store.CountQueued(ctx, "")
				decision := autoscaling.Decide(autoscaling.Inputs{
					QueueDepth: queued, ActiveJobs: active, RequestedCPUUnits: float64(cpu) / 1000,
					CurrentCapacity: current, MinCapacity: 1, MaxCapacity: cfg.GlobalActiveLimit,
					MaxScaleUpStep: 2, MaxScaleDownStep: 1, TargetQueueDrain: 2 * time.Minute,
				})
				current = decision.DesiredCapacity
				telemetry.AutoscalerDesiredCapacity.Set(float64(current))
				logger.Info("autoscaler decision", "desired_capacity", current, "reason", decision.Reason)
			}
		}
	}()
	go func() {
		logger.Info("autoscaler metrics listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}
