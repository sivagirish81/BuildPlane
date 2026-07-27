package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

type Registry struct {
	mu       sync.Mutex
	service  string
	counters map[string]metricSample
	gauges   map[string]metricSample
}

type metricSample struct {
	name   string
	labels map[string]string
	value  float64
}

func NewRegistry(service string) *Registry {
	return &Registry{
		service:  service,
		counters: map[string]metricSample{},
		gauges:   map[string]metricSample{},
	}
}

func (r *Registry) Inc(name string, labels map[string]string) {
	r.Add(name, labels, 1)
}

func (r *Registry) Add(name string, labels map[string]string, value float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	labels = withServiceLabel(r.service, labels)
	key := metricKey(name, labels)
	sample := r.counters[key]
	sample.name = name
	sample.labels = labels
	sample.value += value
	r.counters[key] = sample
}

func (r *Registry) SetGauge(name string, labels map[string]string, value float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	labels = withServiceLabel(r.service, labels)
	key := metricKey(name, labels)
	r.gauges[key] = metricSample{name: name, labels: labels, value: value}
}

func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(r.PrometheusText()))
	})
}

func (r *Registry) PrometheusText() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var builder strings.Builder
	writeSamples(&builder, "counter", r.counters)
	writeSamples(&builder, "gauge", r.gauges)
	return builder.String()
}

func writeSamples(builder *strings.Builder, kind string, samples map[string]metricSample) {
	keys := make([]string, 0, len(samples))
	for key := range samples {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	seen := map[string]bool{}
	for _, key := range keys {
		sample := samples[key]
		if !seen[sample.name] {
			builder.WriteString(fmt.Sprintf("# TYPE %s %s\n", sample.name, kind))
			seen[sample.name] = true
		}
		builder.WriteString(sample.name)
		builder.WriteString(formatLabels(sample.labels))
		builder.WriteString(fmt.Sprintf(" %.6f\n", sample.value))
	}
}

func metricKey(name string, labels map[string]string) string {
	parts := []string{name}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, key+"="+labels[key])
	}
	return strings.Join(parts, "\xff")
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteString("{")
	for index, key := range keys {
		if index > 0 {
			builder.WriteString(",")
		}
		builder.WriteString(key)
		builder.WriteString("=\"")
		builder.WriteString(escapeLabelValue(labels[key]))
		builder.WriteString("\"")
	}
	builder.WriteString("}")
	return builder.String()
}

func withServiceLabel(service string, labels map[string]string) map[string]string {
	copied := map[string]string{"service": service}
	for key, value := range labels {
		copied[key] = value
	}
	return copied
}

func escapeLabelValue(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return value
}
