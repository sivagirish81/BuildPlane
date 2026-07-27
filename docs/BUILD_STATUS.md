# BuildPlane Build Status

Last updated: 2026-07-26

## Current Phase

Phase 3: Queue and Worker Loop

Status: Complete

## Completed Items

- Added durable node execution state in `migrations/0002_queue_worker_loop.sql`.
- Added durable outbox state in `migrations/0002_queue_worker_loop.sql`.
- Updated workflow creation so a new workflow run also creates one initial
  pending node execution in the same PostgreSQL transaction.
- Added scheduler logic that claims schedulable node executions and records
  outbox events.
- Added Redis Streams queue adapter in `services/control-plane/internal/queue`.
- Added worker logic for Redis consumption, PostgreSQL lease acquisition,
  heartbeat, idempotent completion, and message acknowledgement.
- Added fencing-token protection for heartbeats and completion.
- Added `buildplane-scheduler` and `buildplane-worker` command binaries.
- Updated the Dockerfile to build API, scheduler, and worker binaries into the
  same image.
- Updated local Compose configuration to include Redis.
- Added local `kind` manifests for Redis, scheduler, and worker Deployments.
- Added ADR 0003 for Redis Streams with PostgreSQL leases and outbox.
- Added the Phase 3 learning artifact at
  `docs/learning/phase-03-queue-worker-loop.md`.
- Updated README, roadmap, and architecture docs for Phase 3.

## Remaining Limitations

- The worker runs a placeholder deterministic node result. Phase 4 will add a
  clearer local workflow definition and node state machine.
- Backpressure is coarse: the scheduler uses a batch size, but does not yet
  account for tenant limits, retry storms, downstream rate limits, or oldest
  task age.
- Redis consumer-group pending entry recovery is basic and should be expanded
  later.
- No live PostgreSQL/Redis integration test was completed because Docker is not
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
- The current Redis Deployment is for local `kind` learning only and is not a
  production Redis design.
- CRDs and operators are deferred until plain Deployments, Services, Jobs, RBAC,
  probes, and worker lifecycle behavior are understood.

## Next Phase

Phase 4: Local End-to-End Workflow

The next phase should replace the placeholder worker behavior with a small
deterministic workflow definition, explicit node state transitions, audit
records, retry semantics, and integration tests when Docker is available.

## Test Evidence

Commands executed:

```bash
gofmt -w services/control-plane/cmd/buildplane-control-plane/main.go services/control-plane/cmd/buildplane-scheduler/main.go services/control-plane/cmd/buildplane-worker/main.go services/control-plane/internal/httpapi/server.go services/control-plane/internal/httpapi/server_test.go services/control-plane/internal/postgres/db.go services/control-plane/internal/postgres/execution_repository.go services/control-plane/internal/postgres/migrate.go services/control-plane/internal/postgres/workflow_repository.go services/control-plane/internal/queue/redis.go services/control-plane/internal/workflows/execution.go services/control-plane/internal/workflows/execution_test.go services/control-plane/internal/workflows/workflows.go services/control-plane/internal/workflows/workflows_test.go
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go mod tidy
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go test ./services/control-plane/...
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-control-plane ./services/control-plane/cmd/buildplane-control-plane
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-scheduler ./services/control-plane/cmd/buildplane-scheduler
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-worker ./services/control-plane/cmd/buildplane-worker
env -u BUILDPLANE_DATABASE_URL -u BUILDPLANE_REDIS_URL GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go run ./services/control-plane/cmd/buildplane-scheduler
env -u BUILDPLANE_DATABASE_URL -u BUILDPLANE_REDIS_URL GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go run ./services/control-plane/cmd/buildplane-worker
docker compose -f deploy/docker/docker-compose.postgres.yaml config
docker compose -f deploy/docker/docker-compose.postgres.yaml up -d
docker build -f deploy/docker/control-plane.Dockerfile -t buildplane/control-plane:dev .
ruby -e 'require "yaml"; %w[deploy/kind/buildplane-control-plane.yaml deploy/kind/buildplane-redis.yaml deploy/kind/buildplane-scheduler-worker.yaml].each { |path| docs = YAML.load_stream(File.read(path)); puts "#{path}: #{docs.map { |d| d.fetch("kind") }.join(",")}" }'
ruby -e 'sql = File.read("migrations/0002_queue_worker_loop.sql"); %w[node_executions outbox_events fencing_token lease_expires_at].each { |needle| abort "missing #{needle}" unless sql.include?(needle) }; puts "migration contains node_executions, outbox_events, leases, and fencing"'
git diff --check
```

Results:

- Go formatting completed.
- `go mod tidy` completed after network approval to download and verify the
  Redis client dependency.
- Go unit tests passed.
- Go binary builds passed for API, scheduler, and worker using a repo-local Go
  build cache.
- Scheduler startup without `BUILDPLANE_DATABASE_URL` failed as expected.
- Worker startup without `BUILDPLANE_DATABASE_URL` failed as expected.
- Docker Compose config validation passed.
- Starting PostgreSQL and Redis with Docker Compose failed because Docker daemon
  access failed.
- Docker image build failed because Docker daemon access failed.
- Offline YAML parsing passed for control-plane, Redis, scheduler, and worker
  manifests.
- Offline migration check passed and confirmed `node_executions`,
  `outbox_events`, `lease_expires_at`, and `fencing_token`.
- `git diff --check` passed.

## Proposed Commit Message

`feat: add queue and worker lease loop`
