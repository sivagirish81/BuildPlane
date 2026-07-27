# BuildPlane Build Status

Last updated: 2026-07-27

## Current Phase

Phase 11: Frontend Operational UX

Status: Complete

## Completed Items

- Added a Vite, React, and TypeScript operations console under `web/`.
- Added workflow list/detail views, synthetic demo run actions, audit timeline
  rendering, and human approval/rejection controls.
- Added component release views for `issue_classifier`, including dependency
  visibility, component version history, candidate creation, evaluation,
  canary, promotion, and rollback actions.
- Added `GET /v1/workflow-runs` for recent workflow runs.
- Added `GET /v1/workflow-runs/{id}/events` for workflow SSE snapshots.
- Added `GET /v1/component-versions?component_name=...` for recent component
  version state.
- Added frontend typecheck, unit test, and production build commands.
- Added frontend CI checks to the existing pull-request and `main` workflow.
- Added ADR 0011 for the frontend operational console.
- Added the Phase 11 learning artifact at
  `docs/learning/phase-11-frontend-operational-ux.md`.
- Updated README, roadmap, and architecture docs for Phase 11.

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
  queue, SLA timer, comment thread, or notification model exists yet.
- Guarded external actions are mock-only and do not call invoice, freight,
  ticketing, payment, or carrier systems.
- Component versions are durable, but workflow execution still uses the
  app-defined `classify_issue` implementation directly. Live routing by
  component version is deferred.
- Canary percentage is stored as desired release state. It does not yet split
  live workflow traffic or prove production canary health.
- The frontend is a local operational console. It does not yet include auth,
  tenant isolation, server-side sessions, role-aware action guards, persisted UI
  preferences, or deployment manifests.
- SSE streams workflow snapshots only. It does not yet provide a global event
  stream, notification fanout, or durable event replay cursor.
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
  running, Phases 1 through 11 should be exercised manually in a real local
  `kind` cluster or local Compose stack using the documented commands.
- The migration runner is intentionally small. It should be revisited before
  complex schema evolution, rollbacks, checksums, or multi-instance migration
  locking are needed.
- The current Redis Deployment is for local `kind` learning only and is not a
  production Redis design.
- Worker pools do not autoscale yet.
- Operator finalizers, owner references, admission webhooks, and leader election
  hardening are deferred.

## Next Phase

No later phase is defined in the current roadmap. The next continuation should
extend `docs/BUILD_ROADMAP.md` with the next learning phase before
implementation continues.

## Test Evidence

Commands executed:

```bash
.cache/ai-service-venv/bin/python -m pytest ai-service/tests
gofmt -w services/control-plane/internal/httpapi/server.go services/control-plane/internal/httpapi/component_routes.go services/control-plane/internal/httpapi/server_test.go services/control-plane/internal/workflows/workflows_test.go services/control-plane/internal/releases/releases_test.go services/control-plane/internal/workflows/workflows.go services/control-plane/internal/releases/releases.go services/control-plane/internal/postgres/workflow_repository.go services/control-plane/internal/postgres/component_repository.go
go test ./services/control-plane/...
go test ./services/operator/...
npm install
npm audit --omit=optional
npm ci
npm run typecheck
npm test
npm run build
docker compose -f deploy/docker/docker-compose.postgres.yaml up -d --build control-plane
curl -s http://localhost:8080/readyz
curl -s 'http://localhost:8080/v1/workflow-runs?limit=5'
curl -s -X POST http://localhost:8080/v1/workflow-runs -H 'Content-Type: application/json' -H 'Idempotency-Key: phase11-smoke-001' -d '{"workflow_name":"demo.invoice-exception","input":{"case_id":"synthetic-inv-case-001","title":"Invoice price mismatch","description":"Synthetic invoice total is higher than purchase order","customer_message":"Please review this invoice before payment","source":"phase11-smoke","invoice_id":"synthetic-inv-001","vendor_name":"Synthetic Vendor","amount_disputed":1250.50}}'
curl -s http://localhost:8080/v1/workflow-runs/71d86097-35d8-4ca3-8c05-fa53a4339080/audit
curl -sN --max-time 1 http://localhost:8080/v1/workflow-runs/71d86097-35d8-4ca3-8c05-fa53a4339080/events
curl -s 'http://localhost:8080/v1/component-versions?component_name=issue_classifier&limit=5'
curl -s http://localhost:8080/v1/components/issue_classifier/affected-workflows
ruby -e 'require "yaml"; %w[deploy/kind/buildplane-control-plane.yaml deploy/kind/buildplane-config.yaml deploy/kind/buildplane-rbac.yaml deploy/kind/buildplane-redis.yaml deploy/kind/buildplane-scheduler-worker.yaml deploy/kind/buildplane-ai-service.yaml deploy/kind/buildplane-runtime-crd.yaml deploy/kind/buildplane-operator.yaml deploy/kind/buildplane-runtime-sample.yaml deploy/kind/buildplane-observability.yaml].each { |path| docs = YAML.load_stream(File.read(path)); puts "#{path}: #{docs.map { |d| d.fetch("kind") }.join(",")}" }'
ruby -e 'sql = File.read("migrations/0007_component_evaluation_release_gates.sql"); %w[component_versions component_evaluation_runs component_release_events issue_classifier promoted].each { |needle| abort "missing #{needle}" unless sql.include?(needle) }; puts "migration contains component release metadata"'
ruby -e 'require "json"; JSON.parse(File.read("deploy/grafana/buildplane-overview.json")); JSON.parse(File.read("examples/component-versions/issue-classifier-v2.json")); JSON.parse(File.read("examples/demo-workflows/invoice-exception.json")); JSON.parse(File.read("examples/demo-workflows/freight-exception.json"))'
git diff --check
```

Results:

- Python AI service tests passed.
- Go formatting completed.
- Go unit tests passed for control-plane and operator modules after the new
  repository methods and HTTP routes were added.
- Frontend dependency installation passed with `npm install` and `npm ci`.
- `npm audit --omit=optional` passed with zero known vulnerabilities after
  upgrading the Vite/Vitest toolchain.
- Frontend typecheck, Vitest unit tests, and production build passed.
- Control-plane Docker image rebuilt and the local Compose control-plane
  container restarted successfully.
- Live API smoke tests passed for readiness, workflow list, workflow creation,
  audit records, workflow SSE snapshots, component version list, and affected
  workflow detection.
- Offline YAML parsing passed for control-plane, config, RBAC, Redis,
  scheduler, worker pools, AI service, CRD, operator, runtime sample, and
  observability manifests.
- Offline migration check passed for component release metadata.
- Grafana dashboard and example JSON files parsed.
- `git diff --check` passed.
- Starting the Vite dev server from Codex was blocked because the required
  localhost bind escalation hit the current approval usage limit. The app still
  builds successfully and can be run locally with `cd web && npm run dev`.

## Proposed Commit Message

`feat: add frontend operations console`
