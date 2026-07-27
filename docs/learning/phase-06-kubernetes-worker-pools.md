# Phase 6: Kubernetes Worker Pools

## What You Built

Phase 6 split BuildPlane worker execution into named pools:

```text
general: validate_input, compose_summary
ai:      classify_issue
```

Each `node_executions` row now stores `worker_pool`, so routing is part of the
durable state instead of only a Kubernetes convention.

## The Kubernetes Lesson

A Deployment manages identical Pods. If two kinds of work need different
configuration or resources, use two Deployments.

BuildPlane now has:

- `buildplane-worker-general`
- `buildplane-worker-ai`

The general worker has lower resource settings and no AI service URL. The AI
worker has larger resource settings and receives `BUILDPLANE_AI_SERVICE_URL`.

## ConfigMap Versus Secret

ConfigMaps are for non-secret runtime values:

- Redis URL
- migration directory
- scheduler interval
- worker lease duration
- worker drain timeout
- AI service URL

Secrets are for sensitive values:

- PostgreSQL database URL

Even in local `kind`, keeping this split builds the right reflex.

## ServiceAccounts And RBAC

A ServiceAccount is the Pod's Kubernetes identity. RBAC decides what that
identity can do against the Kubernetes API.

These workers do not need Kubernetes API permissions yet, so Phase 6 gives them
explicit ServiceAccounts and binds them to a Role with no rules. That is useful
because later phases can add permissions deliberately instead of inheriting
accidental defaults.

## Graceful Worker Termination

When Kubernetes terminates a Pod, it sends SIGTERM and waits before forcefully
killing the container.

BuildPlane workers now read:

```text
BUILDPLANE_WORKER_DRAIN_TIMEOUT=25s
```

The loop stops taking new work after shutdown starts, but lets the current
iteration finish within the drain timeout. The Pod has
`terminationGracePeriodSeconds: 35`, which is longer than the drain timeout.

That relationship matters:

```text
drain timeout < lease duration < termination grace period is usually safest
```

For this phase:

```text
25s < 30s < 35s
```

The lease duration is still the durable recovery backstop if a Pod dies mid-node.

## Inspect It

After Docker and `kind` are available:

```bash
kubectl apply -f deploy/kind/buildplane-control-plane.yaml
kubectl apply -f deploy/kind/buildplane-config.yaml
kubectl apply -f deploy/kind/buildplane-rbac.yaml
kubectl apply -f deploy/kind/buildplane-redis.yaml
kubectl apply -f deploy/kind/buildplane-ai-service.yaml
kubectl apply -f deploy/kind/buildplane-scheduler-worker.yaml
```

Check the worker pools:

```bash
kubectl get deploy -n buildplane-system -l app.kubernetes.io/part-of=buildplane
kubectl describe deploy buildplane-worker-general -n buildplane-system
kubectl describe deploy buildplane-worker-ai -n buildplane-system
```

Check identities:

```bash
kubectl get serviceaccount -n buildplane-system
kubectl auth can-i list pods --as system:serviceaccount:buildplane-system:buildplane-worker-ai -n buildplane-system
```

Check durable routing:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select node_name, worker_pool, status from node_executions order by created_at;'
```
