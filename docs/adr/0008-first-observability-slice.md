# ADR 0008: First Observability Slice

Date: 2026-07-27

## Status

Accepted

## Context

BuildPlane now has multiple processes: API, scheduler, worker pools, AI service,
and an operator. Debugging by reading individual logs is no longer enough.

The roadmap calls for OpenTelemetry traces, Prometheus metrics, Grafana
dashboards, structured logs, and correlation IDs across services. The first
observability phase should teach those concepts without introducing a full
collector stack before the basics are inspectable.

## Decision

Add a first observability layer:

- Prometheus text metrics from the Go API, scheduler, worker, Python AI service,
  and existing operator metrics endpoint.
- A lightweight in-repo Go metrics registry to avoid hiding the metric format.
- Python AI service metrics implemented with standard library primitives.
- `traceparent` propagation from API request through workflow persistence,
  worker execution, and AI service calls.
- Structured log fields for service name, request method/path/status,
  correlation IDs, trace IDs, worker pool, node name, and outcome.
- Local Prometheus and Grafana manifests for `kind`.
- A starter Grafana dashboard JSON artifact.

This phase does not add a full OpenTelemetry SDK or collector. It adopts the
W3C `traceparent` header now so an OpenTelemetry collector can be introduced
later without changing the service boundary.

## Consequences

- Metrics are visible with `curl /metrics` before Prometheus is running.
- Prometheus can scrape stable Kubernetes Services.
- Grafana has a starter dashboard for BuildPlane RED-style signals.
- High-cardinality IDs are kept out of metric labels.
- Per-run debugging still belongs in audit records and structured logs.
- Real distributed traces, exemplars, alert rules, and persistent observability
  storage are deferred.

## Alternatives Considered

Add full OpenTelemetry instrumentation immediately:

- Deferred. It is the right long-term direction, but this phase prioritizes
  learning the signal shapes first.

Use only controller-runtime/Prometheus dependencies everywhere:

- Rejected for the control plane. A tiny registry makes the Prometheus text
  format explicit and avoids unnecessary dependencies in the hot path.

Put workflow IDs in metric labels:

- Rejected. Workflow IDs are high cardinality and belong in logs/audit, not
  metric series labels.

