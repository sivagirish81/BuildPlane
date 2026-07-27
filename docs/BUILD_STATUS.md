# BuildPlane Build Status

Last updated: 2026-07-27

## Current Phase

Phase 8: Observability

Status: Complete

## Completed Items

- Added a small Go Prometheus text metrics registry in
  `services/control-plane/internal/observability`.
- Added Go trace context helpers for W3C `traceparent`.
- Added migration `migrations/0005_workflow_traceparent.sql`.
- Persisted `traceparent` on workflow runs and hydrated it into worker leases.
- Added `/metrics` to the Go API.
- Added separate metrics servers for scheduler and worker processes.
- Added worker node execution metrics and AI client request metrics.
- Added `/metrics`, request metrics, classification metrics, structured log
  events, and traceparent echoing to the Python AI service.
- Added metrics Services for scheduler, worker pools, and operator.
- Added `deploy/kind/buildplane-observability.yaml` with local Prometheus and
  Grafana.
- Added starter Grafana dashboard JSON at
  `deploy/grafana/buildplane-overview.json`.
- Added tests for metrics rendering, traceparent parsing, and AI client
  traceparent propagation.
- Added ADR 0008 for the first observability slice.
- Added the Phase 8 learning artifact at
  `docs/learning/phase-08-observability.md`.
- Updated README, roadmap, and architecture docs for Phase 8.

## Remaining Limitations

- The workflow definition is still compiled into Go code. A durable
  workflow/component model starts later.
- Worker pool assignment is still compiled into Go code. A durable component
  registry can replace this later.
- The operator only reconciles existing Deployment replica counts. It does not
  create/adopt every BuildPlane resource yet.
- The CRD schema is hand-written. Generated deepcopy/CRD code and webhooks are
  deferred.
- The observability stack is a first slice. It does not yet include a full
  OpenTelemetry SDK, collector, spans, exemplars, alert rules, or persistent
  metrics storage.
- The AI service currently has one classifier endpoint. It does not yet expose
  extraction, embeddings, tool-use planning, eval hooks, or provider metrics.
- The OpenAI provider is implemented but not live-tested in this phase.
- Backpressure is coarse: the scheduler uses a batch size, but does not yet
  account for tenant limits, retry storms, downstream rate limits, or oldest
  task age.
- Redis consumer-group pending entry recovery is basic and should be expanded
  later.
- No live PostgreSQL/Redis end-to-end integration test was completed because
  Docker is not currently reachable.
- The Kubernetes manifests reference local images and a database Secret, but a
  live `kind` deployment was not applied because Docker is not currently
  reachable.
- The Docker images were not built because Docker reports that the daemon is not
  running or not reachable at
  `unix:///Users/sivagirish/.docker/run/docker.sock`.
- The local `rg` command is currently broken due a Homebrew `pcre2` dynamic
  library signing/load issue, so repo discovery used `find`.

## Known Risks

- The full BuildPlane vision is large. The roadmap intentionally starts with a
  tiny Kubernetes workload to prevent premature architecture.
- Kubernetes concepts can feel abstract until inspected live. After Docker is
  running, Phases 1 through 8 should be exercised manually in a real local
  `kind` cluster using the documented commands.
- The migration runner is intentionally small. It should be revisited before
  complex schema evolution, rollbacks, checksums, or multi-instance migration
  locking are needed.
- The current Redis Deployment is for local `kind` learning only and is not a
  production Redis design.
- Worker pools do not autoscale yet.
- Operator finalizers, owner references, admission webhooks, and leader election
  hardening are deferred.

## Next Phase

Phase 9: Demo Workflows

The next phase should build synthetic invoice and freight exception workflows
on top of the execution platform, using reusable components, human approval
points, and guarded mock external actions.

## Test Evidence

Commands executed:

```bash
.cache/ai-service-venv/bin/python -m pytest ai-service/tests
gofmt -w services/control-plane/internal/observability/metrics.go services/control-plane/internal/observability/trace.go services/control-plane/internal/observability/server.go services/control-plane/internal/observability/metrics_test.go services/control-plane/internal/httpapi/server.go services/control-plane/cmd/buildplane-control-plane/main.go services/control-plane/cmd/buildplane-scheduler/main.go services/control-plane/cmd/buildplane-worker/main.go services/control-plane/internal/workflows/workflows.go services/control-plane/internal/workflows/execution.go services/control-plane/internal/workflows/definitions.go services/control-plane/internal/workflows/ai_client.go services/control-plane/internal/workflows/ai_client_test.go services/control-plane/internal/postgres/workflow_repository.go services/control-plane/internal/postgres/execution_repository.go
gofmt -w services/operator/api/v1alpha1/groupversion_info.go services/operator/api/v1alpha1/buildplaneruntime_types.go services/operator/cmd/buildplane-operator/main.go services/operator/internal/controller/buildplaneruntime_controller.go services/operator/internal/controller/buildplaneruntime_controller_test.go
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go test ./services/control-plane/...
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go test ./services/operator/...
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-control-plane ./services/control-plane/cmd/buildplane-control-plane
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-scheduler ./services/control-plane/cmd/buildplane-scheduler
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-worker ./services/control-plane/cmd/buildplane-worker
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-operator ./services/operator/cmd/buildplane-operator
docker compose -f deploy/docker/docker-compose.postgres.yaml config
docker compose -f deploy/docker/docker-compose.postgres.yaml up -d
docker build -f deploy/docker/ai-service.Dockerfile -t buildplane/ai-service:dev .
docker build -f deploy/docker/control-plane.Dockerfile -t buildplane/control-plane:dev .
docker build -f deploy/docker/operator.Dockerfile -t buildplane/operator:dev .
ruby -e 'require "yaml"; %w[deploy/kind/buildplane-control-plane.yaml deploy/kind/buildplane-config.yaml deploy/kind/buildplane-rbac.yaml deploy/kind/buildplane-redis.yaml deploy/kind/buildplane-scheduler-worker.yaml deploy/kind/buildplane-ai-service.yaml deploy/kind/buildplane-runtime-crd.yaml deploy/kind/buildplane-operator.yaml deploy/kind/buildplane-runtime-sample.yaml deploy/kind/buildplane-observability.yaml].each { |path| docs = YAML.load_stream(File.read(path)); puts "#{path}: #{docs.map { |d| d.fetch("kind") }.join(",")}" }'
ruby -e 'sql = File.read("migrations/0004_worker_pools.sql"); %w[worker_pool classify_issue node_executions_worker_pool_check idx_node_executions_schedulable_pool].each { |needle| abort "missing #{needle}" unless sql.include?(needle) }; puts "migration contains worker_pool routing metadata"'
ruby -e 'sql = File.read("migrations/0005_workflow_traceparent.sql"); abort "missing traceparent" unless sql.include?("traceparent"); puts "migration contains workflow traceparent metadata"'
git diff --check
```

Results:

- Python AI service tests passed.
- Go formatting completed.
- Go unit tests passed for control-plane and operator modules.
- Go binary builds passed for API, scheduler, worker, and operator using a
  repo-local Go build cache.
- Docker Compose config validation passed.
- Starting the local Docker Compose stack failed because Docker daemon access
  failed.
- Docker image builds failed because Docker daemon access failed.
- Offline YAML parsing passed for control-plane, config, RBAC, Redis,
  scheduler, worker pools, AI service, CRD, operator, runtime sample, and
  observability manifests.
- Offline migration check passed for `worker_pool` routing metadata.
- Offline migration check passed for workflow `traceparent` metadata.
- `git diff --check` passed.

## Proposed Commit Message

`feat: add first observability slice`
