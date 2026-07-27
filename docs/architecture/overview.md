# BuildPlane Architecture Overview

BuildPlane is a Kubernetes-native control plane for reusable AI workflow
components. It will be built incrementally, starting with a tiny Kubernetes
workload and growing toward a durable workflow execution platform.

## Product Boundary

BuildPlane manages:

- Versioned workflow components
- Workflow runs and node execution state
- Worker dispatch and leases
- Human approval checkpoints
- AI extraction and classification calls through a bounded service
- Guarded external tool calls
- Evaluation, canary, promotion, and rollback of reusable components
- Audit records for every meaningful state transition

BuildPlane does not allow an LLM to mutate external systems or Kubernetes
resources directly.

## Initial System Shape

```mermaid
flowchart LR
  Client["Client or UI"] --> API["Go control-plane API"]
  API --> PG["PostgreSQL source of truth"]
  API --> Outbox["Transactional outbox"]
  Outbox --> Queue["Redis Streams or queue"]
  Queue --> Workers["Kubernetes worker pools"]
  Workers --> AI["Python AI service"]
  Workers --> Tools["Typed mock integrations"]
  Workers --> PG
  API --> Audit["Audit history"]
```

## Future Kubernetes Shape

```mermaid
flowchart TB
  User["kubectl / Helm / Terraform"] --> KubeAPI["Kubernetes API server"]
  KubeAPI --> Deployments["Deployments"]
  KubeAPI --> Services["Services"]
  KubeAPI --> Jobs["Jobs"]
  KubeAPI --> Config["ConfigMaps and Secrets"]
  KubeAPI --> CRDs["BuildPlane Custom Resources"]
  Operator["BuildPlane operator"] --> KubeAPI
  Kubelet["kubelet on nodes"] --> Pods["Pods"]
  Deployments --> Pods
  Jobs --> Pods
```

## Durable State Principles

- PostgreSQL stores workflow runs, node runs, component versions, audit records,
  leases, idempotency records, and outbox events.
- Redis may dispatch work, but Redis is not the only durable record of work.
- Workers are assumed to crash at any point.
- Completion must be idempotent.
- At-least-once delivery is expected.
- Stale workers must be fenced from overwriting newer attempts.

## Kubernetes Learning Path

BuildPlane will learn Kubernetes in this order:

1. Container image
2. Pod
3. Deployment and ReplicaSet
4. Service
5. Probes
6. ConfigMap and Secret
7. Job
8. ServiceAccount and RBAC
9. Resource requests and limits
10. Graceful shutdown
11. Autoscaling
12. CRD and operator
13. Helm packaging
14. GKE deployment

This order is deliberate. Operators make more sense after the lower-level
objects are familiar.

## Current Phase 1 Runtime

Phase 1 introduces one running service:

```mermaid
flowchart LR
  Curl["curl localhost:8080"] --> PF["kubectl port-forward"]
  PF --> SVC["Service: buildplane-control-plane"]
  SVC --> Pod1["Pod replica"]
  SVC --> Pod2["Pod replica"]
  Pod1 --> Go["Go HTTP server"]
  Pod2 --> Go
```

The service is intentionally small. It proves the path from Go source code to
container image to Kubernetes Deployment and Service before adding PostgreSQL,
Redis, CRDs, or workers.

## Current Phase 2 Runtime

Phase 2 makes workflow creation durable:

```mermaid
flowchart LR
  Client["Client"] --> API["Go REST API"]
  API --> Validate["Validate JSON and Idempotency-Key"]
  Validate --> Hash["Compute request hash"]
  Hash --> Tx["PostgreSQL transaction"]
  Tx --> Runs["workflow_runs"]
  Tx --> Migrations["schema_migrations"]
  API --> Logs["Structured logs with correlation ID"]
```

The only workflow state transition implemented so far is:

```text
created request -> queued workflow_run
```

Redis, workers, node state machines, and outbox publication begin in later
phases.

## Current Phase 3 Runtime

Phase 3 introduces the first scheduler and worker loop:

```mermaid
flowchart LR
  API["API creates workflow run"] --> PG["PostgreSQL"]
  PG --> Node["pending node_execution"]
  Scheduler["buildplane-scheduler"] --> Claim["claim node in PostgreSQL"]
  Claim --> Outbox["outbox_events"]
  Scheduler --> Redis["Redis Stream"]
  Worker["buildplane-worker"] --> Redis
  Worker --> Lease["DB lease + fencing token"]
  Worker --> Done["idempotent completion"]
  Done --> PG
```

Redis is not authoritative. A Redis message tells a worker what to try; the
PostgreSQL lease decides whether the worker is allowed to run or complete the
task.

## Current Phase 4 Runtime

Phase 4 adds a local deterministic workflow:

```mermaid
flowchart LR
  Create["POST phase4.local-demo"] --> V["validate_input"]
  V --> S["compose_summary"]
  S --> Done["workflow succeeded"]
  V --> Audit["audit_records"]
  S --> Audit
  Done --> Audit
```

Node ordering is application-defined in Go code for now. Completion of one node
creates the next pending node inside the same PostgreSQL transaction. If a node
fails, retry behavior is bounded by that node's `MaxAttempts`.

## Current Phase 5 Runtime

Phase 5 adds a Python AI service behind a Kubernetes Service:

```mermaid
flowchart LR
  Worker["buildplane-worker"] --> DNS["Service DNS: buildplane-ai-service"]
  DNS --> AI["Python FastAPI AI service"]
  AI --> Mock["Mock provider"]
  AI -. optional .-> OpenAI["OpenAI Responses API"]
  Worker --> PG["PostgreSQL node result + audit"]
```

The worker calls `classify_issue` through a typed HTTP client. The AI service is
mock-backed by default, and the OpenAI provider is enabled only through
environment configuration and credentials.

## Current Phase 6 Runtime

Phase 6 splits worker execution into Kubernetes worker pools:

```mermaid
flowchart LR
  PG["PostgreSQL node_executions(worker_pool)"] --> Scheduler["Scheduler"]
  Scheduler --> Outbox["Outbox payload includes worker_pool"]
  Outbox --> GeneralStream["Redis stream: general"]
  Outbox --> AIStream["Redis stream: ai"]
  GeneralStream --> GeneralWorkers["Deployment: buildplane-worker-general"]
  AIStream --> AIWorkers["Deployment: buildplane-worker-ai"]
  AIWorkers --> AIService["Service: buildplane-ai-service"]
  GeneralWorkers --> PG
  AIWorkers --> PG
```

The durable state model is unchanged. Worker pools decide which Pods should see
which queue messages; PostgreSQL leases and fencing still decide which worker is
allowed to execute and complete a node.

## Current Phase 7 Runtime

Phase 7 adds a narrow Kubernetes operator:

```mermaid
flowchart LR
  User["kubectl apply BuildPlaneRuntime"] --> API["Kubernetes API server"]
  API --> CR["BuildPlaneRuntime spec"]
  Operator["buildplane-operator"] --> CR
  Operator --> Scheduler["Deployment: buildplane-scheduler"]
  Operator --> General["Deployment: buildplane-worker-general"]
  Operator --> AI["Deployment: buildplane-worker-ai"]
  Operator --> Status["BuildPlaneRuntime status.conditions"]
```

The operator reconciles only scheduler and worker pool replica counts. It does
not manage workflow execution state, Redis messages, PostgreSQL rows, Secrets,
or AI provider behavior.

## Current Phase 8 Runtime

Phase 8 adds the first observability layer:

```mermaid
flowchart LR
  API["API /metrics"] --> Prom["Prometheus"]
  Scheduler["Scheduler metrics :9090"] --> Prom
  General["General worker metrics :9090"] --> Prom
  AIWorker["AI worker metrics :9090"] --> Prom
  AI["AI service /metrics"] --> Prom
  Operator["Operator metrics :8080"] --> Prom
  Prom --> Grafana["Grafana dashboard"]
  API -. traceparent .-> Worker["Worker lease context"]
  Worker -. traceparent .-> AI
```

Metrics are intentionally bounded by labels like service, status, workflow name,
worker pool, node name, provider, and result. Workflow run IDs stay in audit
records and logs rather than metric labels.

## Current Phase 9 Runtime

Phase 9 adds synthetic demo workflows with a human approval checkpoint:

```mermaid
flowchart LR
  Create["POST demo workflow"] --> Validate["validate_demo_input"]
  Validate --> AI["classify_issue"]
  AI --> Plan["plan_demo_resolution"]
  Plan --> Approval["await_human_approval"]
  Approval --> Waiting["workflow_run: waiting_for_human"]
  Human["POST /decisions"] --> Decision["human_decisions"]
  Decision --> Resume["record_mock_action"]
  Resume --> Summary["compose_demo_summary"]
  Summary --> Done["workflow succeeded"]
  Decision -. rejected .-> Canceled["workflow canceled"]
```

The approval boundary is durable. A worker can pause the run, but only the API
decision endpoint can resume it. Approved decisions create the guarded mock
action node; rejected decisions cancel the workflow. No real external system is
called in this phase.

## Current Phase 10 Runtime

Phase 10 adds release gates for reusable AI components:

```mermaid
flowchart LR
  Candidate["POST component version"] --> Version["component_versions: candidate"]
  Version --> Affected["affected workflow detection"]
  Version --> Eval["synthetic evaluation"]
  Eval --> Results["component_evaluation_runs/results"]
  Results --> Canary["canary state"]
  Canary --> Promote["promote"]
  Promote --> Current["new promoted version"]
  Promote --> Previous["previous version superseded"]
  Current --> Rollback["rollback"]
  Rollback --> Restored["previous version promoted"]
```

The canary percentage is durable desired release state. It does not yet route
live workflow traffic. The important behavior in this phase is the gate:
evaluation must pass before canary, and canary must exist before promotion.
