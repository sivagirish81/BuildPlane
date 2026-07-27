# BuildPlane Build Status

Last updated: 2026-07-27

## Current Phase

Phase 9: Demo Workflows

Status: Complete

## Completed Items

- Added registered demo workflow names:
  `demo.invoice-exception` and `demo.freight-exception`.
- Added shared demo workflow nodes:
  `validate_demo_input`, `plan_demo_resolution`,
  `await_human_approval`, `record_mock_action`, and
  `compose_demo_summary`.
- Enforced required synthetic invoice and freight fields in the demo validation
  node.
- Kept `classify_issue` as the shared AI-backed classifier node across local
  and demo workflows.
- Added an explicit dependency graph helper for application-defined workflow
  definitions.
- Added `waiting_for_human` as a durable workflow run status.
- Added migration `migrations/0006_human_decisions_and_demo_workflows.sql`.
- Added durable `human_decisions` rows with a unique decision key per workflow
  run.
- Added `POST /v1/workflow-runs/{id}/decisions` for approved/rejected human
  decisions.
- Made approved decisions resume a paused run by creating the guarded
  `record_mock_action` node.
- Made rejected decisions cancel the workflow run.
- Passed the latest human decision into worker lease context so mock external
  action nodes can fail closed without approval.
- Tightened workflow creation so unsupported workflow names are rejected at the
  API boundary.
- Added synthetic invoice and freight request payloads in
  `examples/demo-workflows`.
- Added ADR 0009 for demo workflows and human decisions.
- Added the Phase 9 learning artifact at
  `docs/learning/phase-09-demo-workflows.md`.
- Updated README, roadmap, architecture, and older copy-paste examples for the
  registered workflow names.

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
- Human approval is intentionally narrow: no role authorization, assignment
  queue, SLA timer, comment thread, UI, or notification model exists yet.
- Guarded external actions are mock-only and do not call invoice, freight,
  ticketing, payment, or carrier systems.
- The AI service currently has one classifier endpoint. It does not yet expose
  extraction, embeddings, tool-use planning, eval hooks, or provider metrics.
- The OpenAI provider is implemented but not live-tested in this phase.
- Backpressure is coarse: the scheduler uses a batch size, but does not yet
  account for tenant limits, retry storms, downstream rate limits, or oldest
  task age.
- Redis consumer-group pending entry recovery is basic and should be expanded
  later.
- No live PostgreSQL/Redis end-to-end integration test was completed because
  Docker socket access is not currently available in this environment.
- The Kubernetes manifests reference local images and a database Secret, but a
  live `kind` deployment was not applied because Docker is not currently
  reachable.
- The Docker images were not built because Docker reports permission denied
  while connecting to
  `unix:///Users/sivagirish/.docker/run/docker.sock`.
- The local `rg` command is currently broken due a Homebrew `pcre2` dynamic
  library signing/load issue, so repo discovery used `find`.

## Known Risks

- The full BuildPlane vision is large. The roadmap intentionally starts with a
  tiny Kubernetes workload to prevent premature architecture.
- Kubernetes concepts can feel abstract until inspected live. After Docker is
  running, Phases 1 through 9 should be exercised manually in a real local
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

Phase 10: Evaluation, Canary, and Promotion

The next phase should add the first model for safely updating reusable AI
components through versioning, affected workflow detection, evaluation data,
candidate comparison, canary rollout, promotion, and rollback.

## Test Evidence

Commands executed:

```bash
.cache/ai-service-venv/bin/python -m pytest ai-service/tests
gofmt -w services/control-plane/internal/workflows/workflows.go services/control-plane/internal/workflows/definitions.go services/control-plane/internal/workflows/execution.go services/control-plane/internal/workflows/workflows_test.go services/control-plane/internal/workflows/definitions_test.go services/control-plane/internal/workflows/execution_test.go services/control-plane/internal/postgres/workflow_repository.go services/control-plane/internal/postgres/execution_repository.go services/control-plane/internal/httpapi/server.go services/control-plane/internal/httpapi/server_test.go
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
ruby -e 'sql = File.read("migrations/0006_human_decisions_and_demo_workflows.sql"); %w[waiting_for_human human_decisions approved rejected].each { |needle| abort "missing #{needle}" unless sql.include?(needle) }; puts "migration contains human decision metadata"'
if grep -R --exclude=BUILD_STATUS.md '"workflow_name":"invoice-exception-demo"\|"workflow_name":"phase3-demo"' -n README.md docs services examples; then exit 1; else echo "no unsupported placeholder workflow names"; fi
git diff --check
```

Results:

- Python AI service tests passed.
- Go formatting completed.
- Go unit tests passed for control-plane and operator modules.
- Go binary builds passed for API, scheduler, worker, and operator using a
  repo-local Go build cache.
- Docker Compose config validation passed.
- Starting the local Docker Compose stack did not complete; it was interrupted
  after hanging while trying to pull images in this restricted environment.
- Docker image builds failed because Docker socket access returned permission
  denied.
- Offline YAML parsing passed for control-plane, config, RBAC, Redis,
  scheduler, worker pools, AI service, CRD, operator, runtime sample, and
  observability manifests.
- Offline migration check passed for `worker_pool` routing metadata.
- Offline migration check passed for workflow `traceparent` metadata.
- Offline migration check passed for human decision metadata.
- Copy-paste docs check found no unsupported placeholder workflow names.
- `git diff --check` passed.

## Proposed Commit Message

`feat: add demo workflows with human decisions`
