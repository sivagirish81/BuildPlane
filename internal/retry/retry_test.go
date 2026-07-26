package retry

import (
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	if Classify("pod_evicted", 0) != Recoverable {
		t.Fatal("pod eviction should retry")
	}
	if Classify("", 1) != UserFailure {
		t.Fatal("user command failure should not be infra retry")
	}
	if Classify("invalid_repository", 0) != Terminal {
		t.Fatal("invalid repository should be terminal")
	}
}

func TestBackoffCapped(t *testing.T) {
	got := Backoff(2*time.Second, 60*time.Second, 10, "job-a")
	if got > 60*time.Second {
		t.Fatalf("backoff exceeded cap: %s", got)
	}
	if Backoff(2*time.Second, 60*time.Second, 2, "job-a") <= 2*time.Second {
		t.Fatal("expected increasing backoff plus jitter")
	}
}
