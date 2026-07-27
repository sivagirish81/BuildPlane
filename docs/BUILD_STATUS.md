# BuildPlane Build Status

Last updated: 2026-07-26

## Current Phase

Phase 4: Local End-to-End Workflow

Status: Complete

## Completed Items

- Added application-defined local workflow `phase4.local-demo`.
- Added deterministic workflow nodes:
  - `validate_input`
  - `compose_summary`
- Replaced the Phase 3 placeholder worker result with deterministic node
  execution based on workflow name and node name.
- Added durable audit trail schema in
  `migrations/0003_audit_and_local_workflow.sql`.
- Added audit recording for workflow creation, node creation, queueing, running,
  success, retry scheduling, failure, and workflow terminal states.
- Added `GET /v1/workflow-runs/{id}/audit`.
- Updated node completion so successful non-terminal nodes create the next
  pending node execution in the same transaction.
- Updated failure handling so failed nodes retry with `running -> pending` until
  node max attempts are exhausted.
- Added tests for deterministic workflow node execution and audit API response.
- Added ADR 0004 for the application-defined local workflow and audit trail.
- Added the Phase 4 learning artifact at
  `docs/learning/phase-04-local-end-to-end-workflow.md`.
- Updated README, roadmap, and architecture docs for Phase 4.

## Remaining Limitations

- The workflow definition is compiled into Go code. A durable workflow/component
  model starts later.
- The local workflow is intentionally tiny and deterministic; no Python AI
  service exists yet.
- Backpressure is coarse: the scheduler uses a batch size, but does not yet
  account for tenant limits, retry storms, downstream rate limits, or oldest
  task age.
- Redis consumer-group pending entry recovery is basic and should be expanded
  later.
- No live PostgreSQL/Redis end-to-end integration test was completed because
  Docker is not currently reachable.
- The Kubernetes manifest references a database Secret, but a live `kind`
  deployment was not applied because Docker is not currently reachable.
- The Docker image was not built because Docker reports that the daemon is not
  running or not reachable at
  `unix:///Users/sivagirish/.docker/run/docker.sock`.
- The local `rg` command is currently broken due a Homebrew `pcre2` dynamic
  library signing/load issue, so repo discovery used `find`.

## Known Risks

- The full BuildPlane vision is large. The roadmap intentionally starts with a
  tiny Kubernetes workload to prevent premature architecture.
- Kubernetes concepts can feel abstract until inspected live. After Docker is
  running, Phases 1 through 4 should be exercised manually in a real local
  `kind` cluster using the documented commands.
- The migration runner is intentionally small. It should be revisited before
  complex schema evolution, rollbacks, checksums, or multi-instance migration
  locking are needed.
- The current Redis Deployment is for local `kind` learning only and is not a
  production Redis design.
- CRDs and operators are deferred until plain Deployments, Services, Jobs, RBAC,
  probes, and worker lifecycle behavior are understood.

## Next Phase

Phase 5: Python AI Service

The next phase should add a bounded Python FastAPI AI service with Pydantic
schemas, a provider interface for OpenAI Responses API, structured outputs, a
mock provider for tests, and one workflow node that calls the AI service.

## Test Evidence

Commands executed:

```bash
gofmt -w services/control-plane/cmd/buildplane-control-plane/main.go services/control-plane/cmd/buildplane-scheduler/main.go services/control-plane/cmd/buildplane-worker/main.go services/control-plane/internal/httpapi/server.go services/control-plane/internal/httpapi/server_test.go services/control-plane/internal/postgres/audit.go services/control-plane/internal/postgres/db.go services/control-plane/internal/postgres/execution_repository.go services/control-plane/internal/postgres/migrate.go services/control-plane/internal/postgres/workflow_repository.go services/control-plane/internal/queue/redis.go services/control-plane/internal/workflows/definitions.go services/control-plane/internal/workflows/definitions_test.go services/control-plane/internal/workflows/execution.go services/control-plane/internal/workflows/execution_test.go services/control-plane/internal/workflows/workflows.go services/control-plane/internal/workflows/workflows_test.go
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go mod tidy
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go test ./services/control-plane/...
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-control-plane ./services/control-plane/cmd/buildplane-control-plane
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-scheduler ./services/control-plane/cmd/buildplane-scheduler
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-worker ./services/control-plane/cmd/buildplane-worker
docker compose -f deploy/docker/docker-compose.postgres.yaml config
docker compose -f deploy/docker/docker-compose.postgres.yaml up -d
docker build -f deploy/docker/control-plane.Dockerfile -t buildplane/control-plane:dev .
ruby -e 'require "yaml"; %w[deploy/kind/buildplane-control-plane.yaml deploy/kind/buildplane-redis.yaml deploy/kind/buildplane-scheduler-worker.yaml].each { |path| docs = YAML.load_stream(File.read(path)); puts "#{path}: #{docs.map { |d| d.fetch("kind") }.join(",")}" }'
ruby -e 'sql = File.read("migrations/0003_audit_and_local_workflow.sql"); %w[audit_records workflow_run_id event_type actor_type details].each { |needle| abort "missing #{needle}" unless sql.include?(needle) }; puts "migration contains audit_records and audit fields"'
git diff --check
```

Results:

- Go formatting completed.
- `go mod tidy` completed.
- Go unit tests passed.
- Go binary builds passed for API, scheduler, and worker using a repo-local Go
  build cache.
- Docker Compose config validation passed.
- Starting PostgreSQL and Redis with Docker Compose failed because Docker daemon
  access failed.
- Docker image build failed because Docker daemon access failed.
- Offline YAML parsing passed for control-plane, Redis, scheduler, and worker
  manifests.
- Offline migration check passed and confirmed `audit_records` plus expected
  audit fields.
- `git diff --check` passed.

## Proposed Commit Message

`feat: add local end-to-end workflow audit trail`
