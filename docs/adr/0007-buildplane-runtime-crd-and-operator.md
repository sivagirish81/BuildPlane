# ADR 0007: BuildPlaneRuntime CRD and Operator

Date: 2026-07-27

## Status

Accepted

## Context

BuildPlane now runs as plain Kubernetes Deployments, Services, ConfigMaps,
Secrets, ServiceAccounts, and worker pools. The next learning step is to add a
small Kubernetes-native API without hiding the base objects too early.

The project rule still stands: PostgreSQL owns workflow execution state. A
Kubernetes operator should manage Kubernetes runtime shape, not workflow run
truth.

## Decision

Add a namespaced `BuildPlaneRuntime` Custom Resource:

```yaml
apiVersion: buildplane.io/v1alpha1
kind: BuildPlaneRuntime
spec:
  schedulerReplicas: 1
  workerPools:
    general:
      replicas: 2
    ai:
      replicas: 1
```

Add a Go operator using `sigs.k8s.io/controller-runtime`.

The Phase 7 reconciler only manages replica counts for these existing
Deployments:

- `buildplane-scheduler`
- `buildplane-worker-general`
- `buildplane-worker-ai`

The operator watches one namespace through `WATCH_NAMESPACE`, patches
Deployment replica counts when they differ from the `BuildPlaneRuntime` spec,
and writes status conditions plus observed Deployment availability.

## Consequences

- BuildPlane now has a real Kubernetes API extension and reconciliation loop.
- The CRD teaches `spec` versus `status` without taking over the whole runtime.
- The operator has explicit namespaced RBAC instead of broad cluster-admin
  permissions.
- Existing manifests remain understandable and usable without the operator.
- The operator does not create/adopt every BuildPlane resource yet.
- The operator does not write workflow state or talk to PostgreSQL.
- Autoscaling, finalizers, owner references, webhooks, and generated CRD code
  are deferred.

## Alternatives Considered

Have the operator create all Deployments and Services:

- Rejected for the first operator phase. It would hide the plain Kubernetes
  objects that earlier phases deliberately taught.

Use a hand-written Kubernetes client:

- Rejected. `controller-runtime` is the standard Go library for operators and is
  appropriate now that the base Kubernetes object model has been introduced.

Make the CRD model workflows:

- Rejected for now. Workflow execution state belongs in PostgreSQL, and durable
  workflow/component modeling starts later.

