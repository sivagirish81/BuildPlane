# Phase 8: Observability

## What You Built

Phase 8 added the first observability slice across BuildPlane:

- metrics
- structured logs
- trace context propagation
- local Prometheus
- local Grafana dashboard

This is not the final observability stack. It is the first inspectable layer.

## Metrics

Metrics are numbers over time.

Examples:

```text
buildplane_http_requests_total
buildplane_workflow_runs_total
buildplane_scheduler_ticks_total
buildplane_worker_node_executions_total
buildplane_ai_classifications_total
```

The important rule: do not put unbounded values in labels. BuildPlane uses
bounded labels such as status class, workflow name, worker pool, node name, and
result. Workflow IDs stay in logs and audit records.

Inspect locally:

```bash
curl http://localhost:8080/metrics
curl http://localhost:8090/metrics
```

## Logs

Logs explain individual events. Metrics show shape; logs show detail.

BuildPlane logs include fields such as:

- `correlation_id`
- `trace_id`
- `worker_pool`
- `node_name`
- `status`
- `duration_ms`

Inspect in Kubernetes:

```bash
kubectl logs -n buildplane-system deploy/buildplane-control-plane
kubectl logs -n buildplane-system deploy/buildplane-worker-ai
kubectl logs -n buildplane-system deploy/buildplane-ai-service
```

## Trace Context

Trace context connects work across services.

BuildPlane now accepts or creates a `traceparent` header at the API boundary.
That value is persisted with the workflow run, hydrated into worker leases, sent
to the AI service, and returned by the AI service.

This phase does not send spans to a collector yet. The point is to establish the
propagation path first.

## Prometheus

Prometheus scrapes `/metrics` endpoints.

The local manifest scrapes:

- `buildplane-control-plane:80`
- `buildplane-ai-service:8090`
- `buildplane-scheduler-metrics:9090`
- `buildplane-worker-general-metrics:9090`
- `buildplane-worker-ai-metrics:9090`
- `buildplane-operator-metrics:8080`

Apply it:

```bash
kubectl apply -f deploy/kind/buildplane-observability.yaml
```

Port-forward Prometheus:

```bash
kubectl port-forward -n buildplane-system service/buildplane-prometheus 9090:9090
```

Then open:

```text
http://localhost:9090
```

## Grafana

Grafana reads Prometheus and visualizes queries.

Port-forward Grafana:

```bash
kubectl port-forward -n buildplane-system service/buildplane-grafana 3000:3000
```

Then open:

```text
http://localhost:3000
```

The starter dashboard lives at:

```text
deploy/grafana/buildplane-overview.json
```

## First Debugging Exercise

Create a workflow run, then inspect:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: phase8-demo-001' \
  -H 'traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01' \
  -d '{"workflow_name":"phase4.local-demo","input":{"case_id":"synthetic-case-001","customer_message":"Urgent invoice charge dispute needs escalation"}}'
```

Then check:

```bash
curl http://localhost:8080/metrics
kubectl logs -n buildplane-system deploy/buildplane-worker-ai
kubectl logs -n buildplane-system deploy/buildplane-ai-service
```

Look for the same trace ID:

```text
4bf92f3577b34da6a3ce929d0e0e4736
```

