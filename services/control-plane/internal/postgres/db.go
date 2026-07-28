package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(ctx context.Context, databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

func OpenWithRetry(ctx context.Context, databaseURL string, interval time.Duration) (*sql.DB, error) {
	if interval <= 0 {
		interval = time.Second
	}

	var lastErr error
	for {
		db, err := Open(ctx, databaseURL)
		if err == nil {
			return db, nil
		}
		lastErr = err

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("connect to postgres before startup deadline: %w", lastErr)
		case <-timer.C:
		}
	}
}
