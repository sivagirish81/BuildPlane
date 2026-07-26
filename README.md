# BuildPlane

BuildPlane is a Kubernetes-native control plane for composing, versioning,
executing, evaluating, and safely updating reusable AI workflow components.

The project is intentionally built in phases as a learning system for:

- Go backend engineering
- Kubernetes internals and platform engineering
- Distributed workflow execution
- Python AI services
- React and TypeScript operational UX
- PostgreSQL, Redis, Docker, observability, and cloud deployment

BuildPlane is not a prototype or a clone of a vertical workflow product. It is
general infrastructure for safely operating reusable AI-assisted enterprise
workflows.

Start with:

- [Build roadmap](docs/BUILD_ROADMAP.md)
- [Current build status](docs/BUILD_STATUS.md)
- [Architecture overview](docs/architecture/overview.md)

## Phase 1: First Kubernetes Workload

Run the tiny control-plane service locally:

```bash
go test ./services/control-plane/...
go run ./services/control-plane/cmd/buildplane-control-plane
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/version
```

Build the container image:

```bash
docker build -f deploy/docker/control-plane.Dockerfile -t buildplane/control-plane:dev .
```

Run it in a local `kind` cluster:

```bash
kind create cluster --config deploy/kind/cluster.yaml
kind load docker-image buildplane/control-plane:dev --name buildplane
kubectl apply -f deploy/kind/buildplane-control-plane.yaml
kubectl rollout status deployment/buildplane-control-plane -n buildplane-system
kubectl port-forward -n buildplane-system service/buildplane-control-plane 8080:80
```

Then, in another terminal:

```bash
curl http://localhost:8080/version
```
