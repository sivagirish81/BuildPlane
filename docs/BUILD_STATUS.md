# BuildPlane Build Status

Last updated: 2026-07-27

## Current Phase

Phase 10: Evaluation, Canary, and Promotion

Status: Complete

## Completed Items

- Added a release/evaluation domain package in
  `services/control-plane/internal/releases`.
- Added deterministic synthetic evaluation for the reusable
  `issue_classifier` component.
- Added static affected-workflow detection for workflows that use
  `classify_issue`.
- Added migration `migrations/0007_component_evaluation_release_gates.sql`.
- Added `component_versions`, `component_evaluation_runs`,
  `component_evaluation_results`, and `component_release_events`.
- Seeded promoted baseline `issue_classifier@v1`.
- Added release API endpoints for creating candidate versions, running
  evaluation, starting canary, promoting, rolling back, and listing affected
  workflows.
- Added release gates:
  evaluation must pass before canary, and canary must exist before promotion.
- Added transactional promotion and rollback in PostgreSQL.
- Added synthetic candidate request payload at
  `examples/component-versions/issue-classifier-v2.json`.
- Added ADR 0010 for component evaluation, canary, and promotion.
- Added the Phase 10 learning artifact at
  `docs/learning/phase-10-evaluation-canary-promotion.md`.
- Updated README, roadmap, and architecture docs for Phase 10.

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
- Component versions are durable, but workflow execution still uses the
  app-defined `classify_issue` implementation directly. Live routing by
  component version is deferred.
- Canary percentage is stored as desired release state. It does not yet split
  live workflow traffic or prove production canary health.
- The synthetic evaluation dataset is intentionally tiny and should be expanded
  before any real release confidence claims.
- The AI service currently has one classifier endpoint. It does not yet expose
  extraction, embeddings, tool-use planning, eval hooks, or provider metrics.
- The OpenAI provider is implemented but not live-tested in this phase.
- Backpressure is coarse: the scheduler uses a batch size, but does not yet
  account for tenant limits, retry storms, downstream rate limits, or oldest
  task age.
- Redis consumer-group pending entry recovery is basic and should be expanded
  later.
- The Kubernetes manifests reference local images and a database Secret, but a
  live `kind` deployment was not applied in this phase.
- Docker commands require elevated Docker socket access from the Codex sandbox.
- The local `rg` command is currently broken due a Homebrew `pcre2` dynamic
  library signing/load issue, so repo discovery used `find`.

## Known Risks

- The full BuildPlane vision is large. The roadmap intentionally starts with a
  tiny Kubernetes workload to prevent premature architecture.
- Kubernetes concepts can feel abstract until inspected live. After Docker is
  running, Phases 1 through 10 should be exercised manually in a real local
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

Phase 11: Frontend Operational UX

The next phase should add a React and TypeScript UI for observing and operating
workflows, including workflow list/detail views, audit timelines, human
approval UI, and component dependency/release views.

## Test Evidence

Commands executed:

```bash
.cache/ai-service-venv/bin/python -m pytest ai-service/tests
gofmt -w services/control-plane/internal/releases/releases.go services/control-plane/internal/releases/evaluation.go services/control-plane/internal/releases/releases_test.go services/control-plane/internal/workflows/definitions.go services/control-plane/internal/postgres/component_repository.go services/control-plane/internal/httpapi/server.go services/control-plane/internal/httpapi/component_routes.go services/control-plane/cmd/buildplane-control-plane/main.go
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
curl -i http://localhost:8080/readyz
curl -i -X POST http://localhost:8080/v1/component-versions -H 'Content-Type: application/json' --data @examples/component-versions/issue-classifier-v2.json
curl -i http://localhost:8080/v1/components/issue_classifier/affected-workflows
curl -i -X POST http://localhost:8080/v1/component-versions/<component-version-id>/evaluations
curl -i -X POST http://localhost:8080/v1/component-versions/<component-version-id>/canary -H 'Content-Type: application/json' -d '{"percent":10}'
curl -i -X POST http://localhost:8080/v1/component-versions/<component-version-id>/promote
curl -i -X POST http://localhost:8080/v1/components/issue_classifier/rollback -H 'Content-Type: application/json' -d '{"actor_id":"operator-1","reason":"Synthetic rollback smoke test"}'
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres psql -U buildplane -d buildplane -c 'select component_name, version, status, canary_percent, evaluation_passed, previous_promoted_version_id from component_versions order by created_at;'
ruby -e 'require "yaml"; %w[deploy/kind/buildplane-control-plane.yaml deploy/kind/buildplane-config.yaml deploy/kind/buildplane-rbac.yaml deploy/kind/buildplane-redis.yaml deploy/kind/buildplane-scheduler-worker.yaml deploy/kind/buildplane-ai-service.yaml deploy/kind/buildplane-runtime-crd.yaml deploy/kind/buildplane-operator.yaml deploy/kind/buildplane-runtime-sample.yaml deploy/kind/buildplane-observability.yaml].each { |path| docs = YAML.load_stream(File.read(path)); puts "#{path}: #{docs.map { |d| d.fetch("kind") }.join(",")}" }'
ruby -e 'sql = File.read("migrations/0004_worker_pools.sql"); %w[worker_pool classify_issue node_executions_worker_pool_check idx_node_executions_schedulable_pool].each { |needle| abort "missing #{needle}" unless sql.include?(needle) }; puts "migration contains worker_pool routing metadata"'
ruby -e 'sql = File.read("migrations/0005_workflow_traceparent.sql"); abort "missing traceparent" unless sql.include?("traceparent"); puts "migration contains workflow traceparent metadata"'
ruby -e 'sql = File.read("migrations/0006_human_decisions_and_demo_workflows.sql"); %w[waiting_for_human human_decisions approved rejected].each { |needle| abort "missing #{needle}" unless sql.include?(needle) }; puts "migration contains human decision metadata"'
ruby -e 'sql = File.read("migrations/0007_component_evaluation_release_gates.sql"); %w[component_versions component_evaluation_runs component_release_events issue_classifier promoted].each { |needle| abort "missing #{needle}" unless sql.include?(needle) }; puts "migration contains component release metadata"'
ruby -e 'require "json"; JSON.parse(File.read("examples/component-versions/issue-classifier-v2.json")); puts "component version example json parses"'
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
- Docker image builds passed for control-plane, AI service, and operator after
  elevated Docker socket access was granted.
- Local Docker Compose stack started successfully after elevated Docker socket
  access was granted.
- Live Phase 10 smoke test passed: candidate creation, affected workflow
  detection, evaluation, canary, promotion, rollback, and direct PostgreSQL
  state inspection.
- Offline YAML parsing passed for control-plane, config, RBAC, Redis,
  scheduler, worker pools, AI service, CRD, operator, runtime sample, and
  observability manifests.
- Offline migration check passed for `worker_pool` routing metadata.
- Offline migration check passed for workflow `traceparent` metadata.
- Offline migration check passed for human decision metadata.
- Offline migration check passed for component release metadata.
- Component version example JSON parsed.
- Copy-paste docs check found no unsupported placeholder workflow names.
- `git diff --check` passed.

## Proposed Commit Message

`feat: add component evaluation release gates`
