# BuildPlane Build Status

Last updated: 2026-07-26

## Current Phase

Phase 2: Durable Control Plane Skeleton

Status: Complete

## Completed Items

- Added durable workflow-run domain logic in `services/control-plane/internal/workflows`.
- Added `POST /v1/workflow-runs` for idempotent workflow creation.
- Added `GET /v1/workflow-runs/{id}` for workflow status reads.
- Added request correlation IDs through `X-Correlation-ID`, response headers,
  response bodies, and structured request logs.
- Added PostgreSQL connection wiring through `BUILDPLANE_DATABASE_URL`.
- Added startup migration execution through `BUILDPLANE_MIGRATIONS_DIR`.
- Added PostgreSQL repository code in `services/control-plane/internal/postgres`.
- Added `migrations/0001_create_workflow_runs.sql`.
- Added local PostgreSQL Compose configuration in
  `deploy/docker/docker-compose.postgres.yaml`.
- Updated the Kubernetes Deployment to read `BUILDPLANE_DATABASE_URL` from a
  Secret named `buildplane-postgres`.
- Added ADR 0002 for PostgreSQL-backed idempotent workflow creation.
- Added the Phase 2 learning artifact at
  `docs/learning/phase-02-durable-control-plane-skeleton.md`.
- Updated README, roadmap, and architecture docs for Phase 2.

## Remaining Limitations

- No worker dispatch exists yet. Workflow runs stop at `queued`.
- No node state machine exists yet.
- No Redis queue or transactional outbox exists yet.
- No live PostgreSQL integration test was completed because Docker is not
  currently reachable.
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
  running, Phase 1 and Phase 2 should be exercised manually in a real local
  `kind` cluster using the documented commands.
- The migration runner is intentionally small. It should be revisited before
  complex schema evolution, rollbacks, checksums, or multi-instance migration
  locking are needed.
- CRDs and operators are deferred until plain Deployments, Services, Jobs, RBAC,
  probes, and worker lifecycle behavior are understood.

## Next Phase

Phase 3: Queue and Worker Loop

The next phase should add Redis-backed dispatch, a scheduler loop, worker lease
acquisition, heartbeats, idempotent completion, and the first durable outbox
pattern.

## Test Evidence

Commands executed:

```bash
gofmt -w services/control-plane/cmd/buildplane-control-plane/main.go services/control-plane/internal/httpapi/server.go services/control-plane/internal/httpapi/server_test.go services/control-plane/internal/postgres/db.go services/control-plane/internal/postgres/migrate.go services/control-plane/internal/postgres/workflow_repository.go services/control-plane/internal/workflows/workflows.go services/control-plane/internal/workflows/workflows_test.go
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go mod tidy
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go test ./services/control-plane/...
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-control-plane ./services/control-plane/cmd/buildplane-control-plane
env -u BUILDPLANE_DATABASE_URL GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go run ./services/control-plane/cmd/buildplane-control-plane
docker compose -f deploy/docker/docker-compose.postgres.yaml config
docker compose -f deploy/docker/docker-compose.postgres.yaml up -d
docker build -f deploy/docker/control-plane.Dockerfile -t buildplane/control-plane:dev .
ruby -e 'require "yaml"; docs = YAML.load_stream(File.read("deploy/kind/buildplane-control-plane.yaml")); puts docs.map { |d| d.fetch("kind") }.join("\n")'
ruby -e 'sql = File.read("migrations/0001_create_workflow_runs.sql"); abort "missing workflow_runs" unless sql.include?("CREATE TABLE workflow_runs"); abort "missing unique idempotency" unless sql.include?("idempotency_key text NOT NULL UNIQUE"); puts "migration contains workflow_runs and unique idempotency key"'
```

Results:

- Go formatting completed.
- `go mod tidy` completed after network approval to download and verify the pgx
  PostgreSQL driver dependency.
- Go unit tests passed.
- Go binary build passed using a repo-local Go build cache.
- Startup without `BUILDPLANE_DATABASE_URL` failed as expected.
- Docker Compose config validation passed.
- Starting PostgreSQL with Docker Compose failed because Docker daemon access
  failed.
- Docker image build failed because Docker daemon access failed.
- Offline YAML parsing passed and found `Namespace`, `Deployment`, and
  `Service`.
- Offline migration check passed and confirmed `workflow_runs` plus unique
  `idempotency_key`.

## Proposed Commit Message

`feat: add durable workflow run API`
