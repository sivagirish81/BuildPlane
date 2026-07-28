package postgres

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestOpenWithRetryReturnsStartupDeadlineError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := OpenWithRetry(ctx, "postgres://buildplane:buildplane@127.0.0.1:1/buildplane?sslmode=disable", time.Millisecond)
	if err == nil {
		t.Fatal("expected connection error")
	}

	if !strings.Contains(err.Error(), "connect to postgres before startup deadline") {
		t.Fatalf("expected startup deadline error, got %v", err)
	}
}
