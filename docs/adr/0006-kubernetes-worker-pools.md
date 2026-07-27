# ADR 0006: Kubernetes Worker Pools

Date: 2026-07-27

## Status

Accepted

## Context

BuildPlane now has deterministic workflow nodes and one AI-backed node. These
nodes should not all run in an undifferentiated worker Deployment forever.
Different node classes need different resource profiles, failure budgets, and
configuration.

The existing design still requires PostgreSQL to remain authoritative. Redis
can route work, but a queue message must not be the only record of where a node
is supposed to run.

## Decision

Add a durable `worker_pool` field to `node_executions`.

The current pool mapping is:

```text
validate_input -> general
classify_issue -> ai
compose_summary -> general
```

The scheduler includes `worker_pool` in outbox payloads. Redis dispatch writes
to one stream per pool:

```text
buildplane:node-executions:general
buildplane:node-executions:ai
```

Workers read `BUILDPLANE_WORKER_POOL` and consume only the matching stream.
Kubernetes runs `buildplane-worker-general` and `buildplane-worker-ai` as
separate Deployments with explicit ServiceAccounts, resource settings, and
termination grace periods.

Runtime configuration moves into `buildplane-runtime-config`; the database URL
stays in the `buildplane-postgres` Secret.

## Consequences

- Worker routing is durable and inspectable in PostgreSQL.
- AI work can scale separately from deterministic workflow work.
- A worker pool can be restarted without directly affecting other pools.
- The Kubernetes manifests now teach ServiceAccounts, ConfigMaps, Secrets,
  resource requests/limits, and graceful worker termination.
- Pool mapping is still compiled into Go code. A durable component registry can
  replace it later.
- This phase does not introduce autoscaling or Kubernetes Jobs.

