package retry

import (
	"hash/fnv"
	"math"
	"time"
)

type Class string

const (
	Terminal    Class = "terminal"
	Recoverable Class = "recoverable"
	UserFailure Class = "user_failure"
	Cancelled   Class = "cancelled"
)

func Classify(reason string, exitCode int) Class {
	switch reason {
	case "worker_lost", "pod_evicted", "kubernetes_api", "object_storage", "infrastructure", "lease expired":
		return Recoverable
	case "cancelled":
		return Cancelled
	case "invalid_repository", "invalid_workflow":
		return Terminal
	default:
		if exitCode != 0 {
			return UserFailure
		}
		return Terminal
	}
}

func Backoff(base, max time.Duration, attempt int, key string) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	multiplier := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(base) * multiplier)
	if delay > max {
		delay = max
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	jitter := time.Duration(h.Sum32()%1000) * time.Millisecond
	if delay+jitter > max {
		return max
	}
	return delay + jitter
}
