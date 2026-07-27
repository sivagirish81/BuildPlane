package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

const TraceParentHeader = "traceparent"

type traceParentKey struct{}

func ContextWithTraceParent(ctx context.Context, traceparent string) context.Context {
	return context.WithValue(ctx, traceParentKey{}, NormalizeTraceParent(traceparent))
}

func TraceParentFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(traceParentKey{}).(string); ok {
		return value
	}
	return ""
}

func TraceParentFromRequest(r *http.Request) string {
	return NormalizeTraceParent(r.Header.Get(TraceParentHeader))
}

func NewTraceParent() string {
	traceID := randomHex(16)
	spanID := randomHex(8)
	return "00-" + traceID + "-" + spanID + "-01"
}

func NormalizeTraceParent(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "-")
	if len(parts) != 4 {
		return ""
	}
	if len(parts[0]) != 2 || len(parts[1]) != 32 || len(parts[2]) != 16 || len(parts[3]) != 2 {
		return ""
	}
	return strings.ToLower(value)
}

func TraceID(traceparent string) string {
	parts := strings.Split(NormalizeTraceParent(traceparent), "-")
	if len(parts) != 4 {
		return ""
	}
	return parts[1]
}

func randomHex(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		for index := range bytes {
			bytes[index] = byte(index + 1)
		}
	}
	return hex.EncodeToString(bytes)
}
