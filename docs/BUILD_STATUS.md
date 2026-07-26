# BuildPlane Build Status

Last updated: 2026-07-26

## Current Phase

Phase 1: First Kubernetes Workload

Status: Complete

## Completed Items

- Added the first Go service at `services/control-plane`.
- Added `/healthz`, `/readyz`, and `/version` endpoints.
- Added unit tests for the HTTP endpoints.
- Added a root `go.work` file for the control-plane module.
- Added `deploy/docker/control-plane.Dockerfile` for the service image.
- Added `deploy/kind/cluster.yaml` for a local `kind` cluster.
- Added `deploy/kind/buildplane-control-plane.yaml` with Namespace,
  Deployment, Service, probes, resource requests and limits, and basic container
  security settings.
- Updated `README.md` with Phase 1 run commands.
- Updated `docs/architecture/overview.md` with the current Phase 1 runtime
  diagram.
- Added the Phase 1 learning artifact at
  `docs/learning/phase-01-first-kubernetes-workload.md`.

## Remaining Limitations

- The service has no durable state yet. PostgreSQL starts in Phase 2.
- The Kubernetes manifest has been syntax-checked offline, but it was not
  applied to a live cluster because Docker is not currently reachable.
- The Docker image was not built because Docker reports that the daemon is not
  running or not reachable at
  `unix:///Users/sivagirish/.docker/run/docker.sock`.
- `kind create cluster` was attempted and failed because it depends on the
  Docker daemon.
- The local `rg` command is currently broken due a Homebrew `pcre2` dynamic
  library signing/load issue, so repo discovery used `find`.

## Known Risks

- The full BuildPlane vision is large. The roadmap intentionally starts with a
  tiny Kubernetes workload to prevent premature architecture.
- Kubernetes concepts can feel abstract until inspected live. After Docker is
  running, Phase 1 should be exercised manually in a real local `kind` cluster
  using the documented commands.
- CRDs and operators are deferred until plain Deployments, Services, Jobs, RBAC,
  probes, and worker lifecycle behavior are understood.

## Next Phase

Phase 2: Durable Control Plane Skeleton

The next phase should add a Go REST API backed by PostgreSQL with migrations,
workflow creation using an idempotency key, and basic workflow status reads.

## Test Evidence

Commands executed:

```bash
gofmt -w services/control-plane/cmd/buildplane-control-plane/main.go services/control-plane/internal/httpapi/server.go services/control-plane/internal/httpapi/server_test.go
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build go test ./services/control-plane/...
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build go build -o .cache/bin/buildplane-control-plane ./services/control-plane/cmd/buildplane-control-plane
env GOCACHE=/Users/sivagirish/Documents/Work/Project/BuildPlane/.cache/go-build go run ./services/control-plane/cmd/buildplane-control-plane
curl -sS http://localhost:8080/healthz
curl -sS http://localhost:8080/readyz
curl -sS http://localhost:8080/version
docker version
docker build -f deploy/docker/control-plane.Dockerfile -t buildplane/control-plane:dev .
kind version
kind create cluster --config deploy/kind/cluster.yaml
kubectl version --client
kubectl apply --dry-run=client --validate=false -f deploy/kind/buildplane-control-plane.yaml
ruby -e 'require "yaml"; docs = YAML.load_stream(File.read("deploy/kind/buildplane-control-plane.yaml")); abort "no docs" if docs.empty?; puts docs.map { |d| d.fetch("kind") }.join("\n")'
```

Results:

- Go formatting completed.
- Go unit tests passed.
- Go binary build passed using a repo-local Go build cache.
- Local service endpoint checks passed:
  - `/healthz` returned `{"status":"ok"}`.
  - `/readyz` returned `{"status":"ready"}`.
  - `/version` returned
    `{"name":"buildplane-control-plane","version":"dev"}`.
- Docker CLI is installed, but Docker daemon access failed.
- `kind` CLI is installed, but cluster creation failed because Docker daemon
  access failed.
- `kubectl` client is installed, but dry-run against the current context failed
  because the configured API server at `127.0.0.1:58146` refused connection.
- Offline YAML parsing passed and found `Namespace`, `Deployment`, and
  `Service`.

## Proposed Commit Message

`feat: add first kubernetes control plane workload`
