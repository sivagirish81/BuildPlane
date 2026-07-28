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

## Phase 2: Durable Workflow Runs

Start local PostgreSQL:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml up -d
```

Run the control plane against PostgreSQL:

```bash
export BUILDPLANE_DATABASE_URL='postgres://buildplane:buildplane_dev_password@localhost:5432/buildplane?sslmode=disable'
env GOCACHE=$PWD/.cache/go-build go run ./services/control-plane/cmd/buildplane-control-plane
```

Create a workflow run:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-001' \
  -H 'X-Correlation-ID: local-demo-001' \
  -d '{"workflow_name":"phase4.local-demo","input":{"case_id":"synthetic-case-001"}}'
```

Retry the same request with the same `Idempotency-Key`; it should return the
same workflow run with `"replayed": true`.

Inspect the database:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select id, workflow_name, status, idempotency_key, created_at from workflow_runs;'
```

## Phase 3: Queue and Worker Loop

Phase 3 adds Redis Streams dispatch, a scheduler, worker leases, heartbeats, and
idempotent completion.

Start local dependencies:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml up -d
export BUILDPLANE_DATABASE_URL='postgres://buildplane:buildplane_dev_password@localhost:5432/buildplane?sslmode=disable'
export BUILDPLANE_REDIS_URL='redis://localhost:6379/0'
```

Run the three processes in separate terminals:

```bash
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod \
  go run ./services/control-plane/cmd/buildplane-control-plane
```

```bash
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod \
  go run ./services/control-plane/cmd/buildplane-scheduler
```

```bash
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod \
  BUILDPLANE_WORKER_ID=local-worker-1 \
  go run ./services/control-plane/cmd/buildplane-worker
```

Create a workflow run:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: phase3-demo-001' \
  -d '{"workflow_name":"phase4.local-demo","input":{"case_id":"synthetic-case-001"}}'
```

Inspect durable state:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select id, workflow_run_id, node_name, status, attempt, lease_worker_id, fencing_token from node_executions;'
```

## Phase 4: Local End-to-End Workflow

Phase 4 replaces the placeholder worker result with a two-node deterministic
workflow named `phase4.local-demo`.

Create a local workflow run:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: phase4-demo-001' \
  -d '{"workflow_name":"phase4.local-demo","input":{"case_id":"synthetic-case-001"}}'
```

Inspect the audit trail:

```bash
curl -i http://localhost:8080/v1/workflow-runs/<workflow-run-id>/audit
```

## Phase 10: Evaluation, Canary, and Promotion

Phase 10 adds a release-safety loop for the reusable `issue_classifier`
component:

```text
candidate -> evaluated -> canary -> promoted
```

Create a candidate component version:

```bash
curl -i -X POST http://localhost:8080/v1/component-versions \
  -H 'Content-Type: application/json' \
  --data @examples/component-versions/issue-classifier-v2.json
```

Check which workflows are affected:

```bash
curl -i http://localhost:8080/v1/components/issue_classifier/affected-workflows
```

Run the synthetic evaluation dataset:

```bash
curl -i -X POST http://localhost:8080/v1/component-versions/<component-version-id>/evaluations
```

Start canary, promote, and rollback:

```bash
curl -i -X POST http://localhost:8080/v1/component-versions/<component-version-id>/canary \
  -H 'Content-Type: application/json' \
  -d '{"percent":10}'

curl -i -X POST http://localhost:8080/v1/component-versions/<component-version-id>/promote

curl -i -X POST http://localhost:8080/v1/components/issue_classifier/rollback \
  -H 'Content-Type: application/json' \
  -d '{"actor_id":"operator-1","reason":"Synthetic rollback exercise"}'
```

The expected node order is:

```text
validate_input -> compose_summary
```

Each meaningful transition is also recorded in `audit_records`.

## Phase 11: Frontend Operational UX

Phase 11 adds a React and TypeScript operations console.

Start the API stack:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml up -d
```

Run the frontend:

```bash
cd web
npm install
npm run dev
```

The Vite dev server proxies `/api` to `http://localhost:8080`. Open the local
URL printed by Vite and use the console to create demo workflow runs, inspect
audit timelines, submit human decisions, and operate the synthetic
`issue_classifier` release flow.

## Phase 12: Cloud Deployment

Phase 12 adds a Helm chart and GKE Terraform scaffold.

Render the Helm chart locally:

```bash
helm lint deploy/helm/buildplane
helm template buildplane deploy/helm/buildplane \
  --namespace buildplane-system \
  --include-crds \
  --values deploy/helm/buildplane/values-gke.example.yaml
```

Plan GKE infrastructure:

```bash
cd infra/gke
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform plan
```

Build the frontend web image:

```bash
docker build -f deploy/docker/web.Dockerfile -t buildplane/web:dev .
```

See [GKE deployment guide](docs/deployment/gke.md) and
[cloud operations runbook](docs/runbooks/cloud-operations.md).

## Phase 5: Python AI Service

Phase 5 inserts a bounded AI classification node into the local workflow:

```text
validate_input -> classify_issue -> compose_summary
```

Create a local Python virtual environment and run the AI service tests:

```bash
python3 -m venv .cache/ai-service-venv
.cache/ai-service-venv/bin/pip install -e 'ai-service[dev]'
.cache/ai-service-venv/bin/python -m pytest ai-service/tests
```

Run the AI service locally with its deterministic mock provider:

```bash
.cache/ai-service-venv/bin/python -m uvicorn app.main:app --app-dir ai-service --host 0.0.0.0 --port 8090
```

In the worker terminal, point the worker at the local AI service:

```bash
export BUILDPLANE_AI_SERVICE_URL='http://localhost:8090'
```

Create a workflow run with issue context:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: phase5-demo-001' \
  -d '{"workflow_name":"phase4.local-demo","input":{"case_id":"synthetic-case-001","customer_message":"Urgent invoice charge dispute needs escalation"}}'
```

Build the AI service image:

```bash
docker build -f deploy/docker/ai-service.Dockerfile -t buildplane/ai-service:dev .
```

Run it in `kind` with the worker:

```bash
kind load docker-image buildplane/ai-service:dev --name buildplane
kubectl apply -f deploy/kind/buildplane-ai-service.yaml
kubectl apply -f deploy/kind/buildplane-scheduler-worker.yaml
```

## Phase 6: Kubernetes Worker Pools

Phase 6 splits worker execution into separate Kubernetes worker pools:

```text
general -> validate_input, compose_summary
ai      -> classify_issue
```

Apply the shared runtime config and explicit Kubernetes identities before the
worker Deployments:

```bash
kubectl apply -f deploy/kind/buildplane-config.yaml
kubectl apply -f deploy/kind/buildplane-rbac.yaml
kubectl apply -f deploy/kind/buildplane-scheduler-worker.yaml
```

Inspect the pools:

```bash
kubectl get deploy -n buildplane-system \
  -l app.kubernetes.io/part-of=buildplane

kubectl describe deploy buildplane-worker-general -n buildplane-system
kubectl describe deploy buildplane-worker-ai -n buildplane-system
```

Inspect the durable routing field:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select node_name, worker_pool, status from node_executions order by created_at;'
```

## Phase 7: Operator and Custom Resources

Phase 7 adds a narrow `BuildPlaneRuntime` CRD and Go operator.

Build and load the operator image:

```bash
docker build -f deploy/docker/operator.Dockerfile -t buildplane/operator:dev .
kind load docker-image buildplane/operator:dev --name buildplane
```

Apply the CRD, operator, and sample runtime:

```bash
kubectl apply -f deploy/kind/buildplane-runtime-crd.yaml
kubectl apply -f deploy/kind/buildplane-operator.yaml
kubectl apply -f deploy/kind/buildplane-runtime-sample.yaml
```

Inspect the custom resource:

```bash
kubectl get buildplaneruntime -n buildplane-system
kubectl get buildplaneruntime local-runtime -n buildplane-system -o yaml
```

Change desired worker pool size:

```bash
kubectl patch buildplaneruntime local-runtime -n buildplane-system \
  --type merge \
  -p '{"spec":{"workerPools":{"ai":{"replicas":2}}}}'
```

Then inspect the reconciled Deployment:

```bash
kubectl get deploy buildplane-worker-ai -n buildplane-system
```

## Phase 8: Observability

Phase 8 adds `/metrics`, trace context propagation, Prometheus, and Grafana.

Inspect service metrics locally:

```bash
curl http://localhost:8080/metrics
curl http://localhost:8090/metrics
```

Apply the local observability stack:

```bash
kubectl apply -f deploy/kind/buildplane-observability.yaml
```

Port-forward Prometheus and Grafana:

```bash
kubectl port-forward -n buildplane-system service/buildplane-prometheus 9090:9090
kubectl port-forward -n buildplane-system service/buildplane-grafana 3000:3000
```

The starter Grafana dashboard is stored at:

```text
deploy/grafana/buildplane-overview.json
```

Create a traced workflow run:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: phase8-demo-001' \
  -H 'traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01' \
  -d '{"workflow_name":"phase4.local-demo","input":{"case_id":"synthetic-case-001","customer_message":"Urgent invoice charge dispute needs escalation"}}'
```

## Phase 9: Demo Workflows

Phase 9 adds synthetic invoice and freight exception workflows with a human
approval checkpoint:

```text
validate_demo_input -> classify_issue -> plan_demo_resolution -> await_human_approval -> record_mock_action -> compose_demo_summary
```

Create the invoice demo:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: invoice-demo-001' \
  --data @examples/demo-workflows/invoice-exception.json
```

Create the freight demo:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: freight-demo-001' \
  --data @examples/demo-workflows/freight-exception.json
```

When a run reaches `waiting_for_human`, approve it:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs/<workflow-run-id>/decisions \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: invoice-demo-approval-001' \
  -d '{"decision":"approved","actor_id":"operator-1","reason":"Synthetic approval for the learning demo"}'
```

Inspect the audit trail:

```bash
curl -i http://localhost:8080/v1/workflow-runs/<workflow-run-id>/audit
```
