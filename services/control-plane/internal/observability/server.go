package observability

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

func StartMetricsServer(ctx context.Context, addr string, registry *Registry, logger *slog.Logger) <-chan error {
	errCh := make(chan error, 1)
	if addr == "" {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           registry.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("starting metrics server", "addr", addr)
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("metrics server shutdown failed", "error", err)
		}
	}()

	return errCh
}
