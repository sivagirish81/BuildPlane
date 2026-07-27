# BuildPlane Build Status

Last updated: 2026-07-27

## Current Phase

Phase 5: Python AI Service

Status: Complete

## Completed Items

- Added `ai-service/`, a Python FastAPI service for bounded AI classification.
- Added Pydantic schemas for classification requests, evidence, and structured
  classification responses.
- Added a deterministic mock AI provider for local development and tests.
- Added an optional OpenAI provider behind `BUILDPLANE_AI_PROVIDER=openai`.
- Added prompt version file `ai-service/prompts/issue_classifier_v1.md`.
- Added `POST /v1/classify-issue`, `/healthz`, and `/readyz` to the AI service.
- Added a typed Go `AIClassifier` interface and bounded HTTP client.
- Added workflow node `classify_issue` to `phase4.local-demo`.
- Updated the worker to call the AI service using `BUILDPLANE_AI_SERVICE_URL`.
- Added Docker and Kubernetes manifests for `buildplane-ai-service`.
- Updated worker Kubernetes env wiring to call the AI service through Service
  DNS.
- Added ADR 0005 for the bounded Python AI service.
- Added the Phase 5 learning artifact at
  `docs/learning/phase-05-python-ai-service.md`.
- Updated README, roadmap, and architecture docs for Phase 5.

## Remaining Limitations

- The workflow definition is still compiled into Go code. A durable
  workflow/component model starts later.
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
  running, Phases 1 through 5 should be exercised manually in a real local
  `kind` cluster using the documented commands.
- The migration runner is intentionally small. It should be revisited before
  complex schema evolution, rollbacks, checksums, or multi-instance migration
  locking are needed.
- The current Redis Deployment is for local `kind` learning only and is not a
  production Redis design.
- CRDs and operators are deferred until plain Deployments, Services, Jobs, RBAC,
  probes, and worker lifecycle behavior are understood.

## Next Phase

Phase 6: Kubernetes Worker Pools

The next phase should split worker concerns into Kubernetes worker pools with
clear scheduling, resource controls, ConfigMaps, Secrets, ServiceAccounts, RBAC,
and graceful shutdown behavior.

## Test Evidence

Commands executed:

```bash
python3 -m venv .cache/ai-service-venv
.cache/ai-service-venv/bin/pip install -e 'ai-service[dev]'
.cache/ai-service-venv/bin/python -m pytest ai-service/tests
gofmt -w services/control-plane/cmd/buildplane-worker/main.go services/control-plane/internal/workflows/ai_client.go services/control-plane/internal/workflows/definitions.go services/control-plane/internal/workflows/definitions_test.go services/control-plane/internal/workflows/execution.go services/control-plane/internal/workflows/execution_test.go
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go test ./services/control-plane/...
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-control-plane ./services/control-plane/cmd/buildplane-control-plane
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-scheduler ./services/control-plane/cmd/buildplane-scheduler
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build GOMODCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-mod go build -o .cache/bin/buildplane-worker ./services/control-plane/cmd/buildplane-worker
docker compose -f deploy/docker/docker-compose.postgres.yaml config
docker compose -f deploy/docker/docker-compose.postgres.yaml up -d
docker build -f deploy/docker/ai-service.Dockerfile -t buildplane/ai-service:dev .
docker build -f deploy/docker/control-plane.Dockerfile -t buildplane/control-plane:dev .
ruby -e 'require "yaml"; %w[deploy/kind/buildplane-control-plane.yaml deploy/kind/buildplane-redis.yaml deploy/kind/buildplane-scheduler-worker.yaml deploy/kind/buildplane-ai-service.yaml].each { |path| docs = YAML.load_stream(File.read(path)); puts "#{path}: #{docs.map { |d| d.fetch("kind") }.join(",")}" }'
git diff --check
```

Results:

- Python AI service tests passed.
- Go formatting completed.
- Go unit tests passed.
- Go binary builds passed for API, scheduler, and worker using a repo-local Go
  build cache.
- Docker Compose config validation passed.
- Starting PostgreSQL, Redis, and the AI service with Docker Compose failed
  because Docker daemon access failed.
- Docker image builds failed because Docker daemon access failed.
- Offline YAML parsing passed for control-plane, Redis, scheduler, worker, and
  AI service manifests.
- `git diff --check` passed.

## Proposed Commit Message

`feat: add bounded python ai service`
