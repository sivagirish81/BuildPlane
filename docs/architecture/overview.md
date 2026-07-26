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
