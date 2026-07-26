# ADR 0001: Pivot to Kubernetes-Native AI Workflow Control Plane

Date: 2026-07-26

## Status

Accepted

## Context

The repository previously described BuildPlane as a distributed CI orchestration
platform for containerized build and test workloads across Kubernetes clusters.

The project is being repurposed into a learning-focused but production-minded
open-source system for Kubernetes-native AI workflow operations.

The user wants to learn Kubernetes deeply by building a real control-plane
style system rather than studying isolated examples.

## Decision

BuildPlane will become a Kubernetes-native control plane for composing,
versioning, executing, evaluating, and safely updating reusable AI workflow
components.

The system will be built in phases. Early phases will use simple Kubernetes
objects directly. Custom Resources and a Go operator will be introduced only
after Deployments, Services, Jobs, probes, RBAC, worker lifecycles, and durable
workflow execution are understood.

The selected core stack is:

- Go for the control plane and Kubernetes operator
- PostgreSQL as the durable source of truth
- Redis Streams or Redis-backed queues for dispatch and short-lived coordination
- Python FastAPI for bounded AI service behavior
- React and TypeScript for operational UX
- Docker, `kind`, Helm, Terraform, and GKE for infrastructure learning
- OpenTelemetry, Prometheus, Grafana, and structured logs for observability

## Consequences

Positive:

- The project becomes a coherent learning vehicle for Kubernetes, distributed
  systems, and AI workflow infrastructure.
- The roadmap can teach Kubernetes from first principles through operators.
- The architecture supports production concepts such as idempotency, leases,
  audit histories, and controlled rollout.

Negative:

- The scope is large and must be managed strictly by phases.
- The first visible product features will arrive later because infrastructure
  foundations come first.
- Documentation and checkpoints are mandatory to avoid losing the learning
  thread.

## Alternatives Considered

Start with a Kubernetes operator immediately:

- Rejected for now. Operators are important, but starting there would hide too
  much of the base Kubernetes model.

Start with the AI demo workflows:

- Rejected for now. The demos should sit on a functional execution platform,
  not define the platform accidentally.

Keep the CI orchestration direction:

- Rejected. The new goal is broader AI workflow infrastructure and deeper
  Kubernetes learning.
