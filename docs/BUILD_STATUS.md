# BuildPlane Build Status

Last updated: 2026-07-27

## Current Phase

Phase 6: Kubernetes Worker Pools

Status: Complete

## Completed Items

- Added durable `worker_pool` metadata to `node_executions`.
- Added migration `migrations/0004_worker_pools.sql`.
- Added worker pool mapping:
  - `validate_input` -> `general`
  - `classify_issue` -> `ai`
  - `compose_summary` -> `general`
- Updated scheduler outbox payloads to include `worker_pool`.
- Updated Redis dispatch to use one stream per worker pool.
- Updated the worker to read `BUILDPLANE_WORKER_POOL`.
- Added worker drain timeout handling through `BUILDPLANE_WORKER_DRAIN_TIMEOUT`.
- Split Kubernetes workers into `buildplane-worker-general` and
  `buildplane-worker-ai` Deployments.
- Added `deploy/kind/buildplane-config.yaml` for non-secret runtime config.
- Added `deploy/kind/buildplane-rbac.yaml` with explicit ServiceAccounts and
  no-permission RoleBindings.
- Updated Kubernetes manifests to use ConfigMaps, Secrets, ServiceAccounts,
  resource settings, and termination grace periods.
- Updated Docker Compose with local API, scheduler, general worker, and AI
  worker services.
- Added ADR 0006 for Kubernetes worker pools.
- Added the Phase 6 learning artifact at
  `docs/learning/phase-06-kubernetes-worker-pools.md`.
- Updated README, roadmap, and architecture docs for Phase 6.

## Remaining Limitations

- The workflow definition is still compiled into Go code. A durable
  workflow/component model starts later.
- Worker pool assignment is still compiled into Go code. A durable component
  registry can replace this later.
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
  running, Phases 1 through 6 should be exercised manually in a real local
  `kind` cluster using the documented commands.
- The migration runner is intentionally small. It should be revisited before
  complex schema evolution, rollbacks, checksums, or multi-instance migration
  locking are needed.
- The current Redis Deployment is for local `kind` learning only and is not a
  production Redis design.
- Worker pools do not autoscale yet.
- CRDs and operators are deferred until plain Deployments, Services, Jobs, RBAC,
  probes, and worker lifecycle behavior are understood.

## Next Phase

Phase 7: Operator and Custom Resources

The next phase should introduce a narrow Kubernetes Custom Resource and a Go
controller reconciliation loop after the plain Deployment, Service,
ConfigMap/Secret, RBAC, and worker lifecycle concepts have been exercised.

## Test Evidence

Commands executed:

```bash
.cache/ai-service-venv/bin/python -m pytest ai-service/tests
gofmt -w services/control-plane/cmd/buildplane-worker/main.go services/control-plane/internal/postgres/execution_repository.go services/control-plane/internal/postgres/workflow_repository.go services/control-plane/internal/queue/redis.go services/control-plane/internal/queue/redis_test.go services/control-plane/internal/workflows/definitions.go services/control-plane/internal/workflows/definitions_test.go services/control-plane/internal/workflows/execution.go
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go test ./services/control-plane/...
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-control-plane ./services/control-plane/cmd/buildplane-control-plane
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-scheduler ./services/control-plane/cmd/buildplane-scheduler
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-worker ./services/control-plane/cmd/buildplane-worker
docker compose -f deploy/docker/docker-compose.postgres.yaml config
docker compose -f deploy/docker/docker-compose.postgres.yaml up -d
docker build -f deploy/docker/ai-service.Dockerfile -t buildplane/ai-service:dev .
docker build -f deploy/docker/control-plane.Dockerfile -t buildplane/control-plane:dev .
ruby -e 'require "yaml"; %w[deploy/kind/buildplane-control-plane.yaml deploy/kind/buildplane-config.yaml deploy/kind/buildplane-rbac.yaml deploy/kind/buildplane-redis.yaml deploy/kind/buildplane-scheduler-worker.yaml deploy/kind/buildplane-ai-service.yaml].each { |path| docs = YAML.load_stream(File.read(path)); puts "#{path}: #{docs.map { |d| d.fetch("kind") }.join(",")}" }'
ruby -e 'sql = File.read("migrations/0004_worker_pools.sql"); %w[worker_pool classify_issue node_executions_worker_pool_check idx_node_executions_schedulable_pool].each { |needle| abort "missing #{needle}" unless sql.include?(needle) }; puts "migration contains worker_pool routing metadata"'
git diff --check
```

Results:

- Python AI service tests passed.
- Go formatting completed.
- Go unit tests passed.
- Go binary builds passed for API, scheduler, and worker using a repo-local Go
  build cache.
- Docker Compose config validation passed.
- Starting the local Docker Compose stack failed because Docker daemon access
  failed.
- Docker image builds failed because Docker daemon access failed.
- Offline YAML parsing passed for control-plane, config, RBAC, Redis,
  scheduler, worker pools, and AI service manifests.
- Offline migration check passed for `worker_pool` routing metadata.
- `git diff --check` passed.

## Proposed Commit Message

`feat: add kubernetes worker pools`
