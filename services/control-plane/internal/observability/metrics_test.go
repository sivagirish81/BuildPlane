package observability

import (
	"strings"
	"testing"
)

func TestRegistryPrometheusText(t *testing.T) {
	registry := NewRegistry("test-service")
	registry.Inc("buildplane_test_events_total", map[string]string{"result": "ok"})
	registry.Add("buildplane_test_events_total", map[string]string{"result": "ok"}, 2)
	registry.SetGauge("buildplane_test_depth", map[string]string{"queue": "general"}, 4)

	text := registry.PrometheusText()
	for _, expected := range []string{
		`# TYPE buildplane_test_events_total counter`,
		`buildplane_test_events_total{result="ok",service="test-service"} 3.000000`,
		`# TYPE buildplane_test_depth gauge`,
		`buildplane_test_depth{queue="general",service="test-service"} 4.000000`,
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in metrics text:\n%s", expected, text)
		}
	}
}

func TestTraceParentNormalization(t *testing.T) {
	traceparent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	if got := NormalizeTraceParent(traceparent); got != traceparent {
		t.Fatalf("expected traceparent preserved, got %q", got)
	}
	if got := TraceID(traceparent); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("expected trace id, got %q", got)
	}
	if got := NormalizeTraceParent("bad"); got != "" {
		t.Fatalf("expected invalid traceparent rejected, got %q", got)
	}
}
