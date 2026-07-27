package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sivagirish/buildplane/services/control-plane/internal/httpapi"
	"github.com/sivagirish/buildplane/services/control-plane/internal/postgres"
	"github.com/sivagirish/buildplane/services/control-plane/internal/workflows"
)

var version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("control plane stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	databaseURL := os.Getenv("BUILDPLANE_DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("BUILDPLANE_DATABASE_URL is required")
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()

	db, err := postgres.Open(startupCtx, databaseURL)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("close postgres", "error", err)
		}
	}()

	migrationsDir := os.Getenv("BUILDPLANE_MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}
	if err := postgres.Migrate(startupCtx, db, migrationsDir); err != nil {
		return err
	}

	workflowRepository := postgres.NewWorkflowRepository(db)
	workflowService := workflows.NewService(workflowRepository)

	server := &http.Server{
		Addr: ":" + port,
		Handler: httpapi.NewServer(httpapi.Options{
			Version:   version,
			Logger:    logger,
			Ready:     workflowRepository.Ping,
			Workflows: workflowService,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting control plane", "addr", server.Addr, "version", version)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		logger.Info("shutting down control plane")
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
