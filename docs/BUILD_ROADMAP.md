# BuildPlane Build Roadmap

This roadmap is intentionally sequential. Each phase teaches one layer, adds a
working slice, records what was learned, and stops until the user says
`CONTINUE`.

## Phase 0: Project Reset and Learning Contract

Status: Complete

Goal: Pivot the repository to the Kubernetes-native AI workflow control plane
direction and create the docs that govern future phases.

Learning focus:

- Kubernetes as an API-driven control plane
- Desired state and controllers
- Why operators exist
- Why we will start with plain Kubernetes objects before CRDs

## Phase 1: First Kubernetes Workload

Status: Complete

Goal: Build the smallest useful BuildPlane service and run it locally in `kind`.

Planned behavior:

- A tiny Go HTTP service with `/healthz`, `/readyz`, and `/version`
- Docker image for the service
- Local `kind` cluster instructions
- Kubernetes Deployment and Service
- Readiness and liveness probes
- Commands to inspect Pods, events, logs, rollout state, and failure modes

Learning focus:

- Containers versus Pods
- Deployments and ReplicaSets
- Services and cluster networking
- Probes
- `kubectl describe`, logs, exec, events, and rollout debugging

## Phase 2: Durable Control Plane Skeleton

Status: Complete

Goal: Add a Go REST API backed by PostgreSQL with migrations.

Planned behavior:

- Create workflow runs with an idempotency key
- Persist workflow state
- Query workflow run status
- Basic structured logs and request correlation IDs

Learning focus:

- REST API boundaries
- PostgreSQL as source of truth
- Migrations
- Idempotency
- Transaction boundaries

## Phase 3: Queue and Worker Loop

Status: Complete

Goal: Dispatch persisted node executions to workers using Redis Streams or a
Redis-backed queue.

Planned behavior:

- Scheduler claims pending work from PostgreSQL
- Queue publication via transactional outbox
- Worker lease acquisition
- Heartbeats
- Idempotent completion

Learning focus:

- At-least-once delivery
- Worker leases and fencing
- Backpressure basics
- Crash recovery

## Phase 4: Local End-to-End Workflow

Status: Complete

Goal: Execute a small deterministic workflow from API request to completed run.

Planned behavior:

- Workflow definition format in application code
- Node state machine
- Deterministic code component
- Complete audit trail
- Integration test with PostgreSQL and Redis

Learning focus:

- Workflow orchestration
- State machines
- Retry semantics
- Auditability

## Phase 5: Python AI Service

Status: Complete

Goal: Add a bounded Python FastAPI service for structured extraction or
classification.

Planned behavior:

- Provider interface for OpenAI Responses API
- Pydantic request and response schemas
- Structured outputs
- Mock provider for tests
- One workflow node that calls the AI service

Learning focus:

- AI service isolation
- Structured outputs
- Prompt versioning
- Deterministic validation around LLM output

## Phase 6: Kubernetes Worker Pools

Goal: Run different worker types in Kubernetes with clear scheduling and
resource controls.

Planned behavior:

- Separate worker Deployments
- Resource requests and limits
- ServiceAccounts
- ConfigMaps and Secrets
- Graceful shutdown and worker draining

Learning focus:

- Scheduling
- Resource management
- RBAC
- Pod lifecycle
- Graceful termination

## Phase 7: Operator and Custom Resources

Goal: Introduce a Go Kubernetes operator only after the plain objects are well
understood.

Planned behavior:

- CRD for a narrow BuildPlane runtime concept
- Controller reconciliation loop
- Status conditions
- RBAC for the operator
- Operator tests

Learning focus:

- CRDs
- Reconcilers
- Informers and caches
- Status versus spec
- Controller failure modes

## Phase 8: Observability

Goal: Add production-grade visibility across the control plane and workers.

Planned behavior:

- OpenTelemetry traces
- Prometheus metrics
- Grafana dashboard
- Structured logs
- Correlation IDs across services

Learning focus:

- Metrics, logs, and traces
- RED and USE signals
- Alerting basics
- Debugging distributed requests

## Phase 9: Demo Workflows

Goal: Build the invoice and freight exception workflows on top of the execution
platform.

Planned behavior:

- Synthetic invoice exception demo
- Synthetic freight exception demo
- Reusable shared components
- Dependency graph
- Human approval point
- Guarded mock external actions

Learning focus:

- Component reuse
- Versioning
- Human-in-the-loop workflows
- Safe external effects

## Phase 10: Evaluation, Canary, and Promotion

Goal: Safely update reusable AI components.

Planned behavior:

- Component version graph
- Affected workflow detection
- Evaluation datasets
- Candidate comparison
- Canary rollout
- Promotion and rollback

Learning focus:

- Compatibility checks
- Eval-driven releases
- Progressive delivery
- Historical reproducibility

## Phase 11: Frontend Operational UX

Goal: Add a React and TypeScript UI for observing and operating workflows.

Planned behavior:

- Workflow list and detail views
- Live updates via SSE or WebSockets
- Audit timeline
- Human approval UI
- Component dependency graph

Learning focus:

- Frontend state
- Data fetching
- Accessibility
- Operational interface design

## Phase 12: Cloud Deployment

Goal: Deploy the platform to GKE with Terraform and Helm.

Planned behavior:

- Helm chart
- Terraform for GKE infrastructure
- Cloud deployment guide
- Operational runbook
- Notes on how the architecture could map to EKS

Learning focus:

- Kubernetes in cloud environments
- Helm packaging
- Terraform state
- Production readiness checks
