# ADR 0003: Redis Streams with PostgreSQL Leases and Outbox

Date: 2026-07-26

## Status

Accepted

## Context

BuildPlane now persists workflow runs in PostgreSQL. The next step is dispatch:
pending work must move to workers without losing durable state or assuming
exactly-once delivery.

Redis Streams are useful for local dispatch and worker coordination, but Redis
must not become the authoritative workflow state store.

## Decision

BuildPlane will use PostgreSQL as the durable source of truth and Redis Streams
as the dispatch mechanism.

The Phase 3 flow is:

1. Workflow creation inserts a `workflow_runs` row and one pending
   `node_executions` row in the same transaction.
2. The scheduler claims schedulable node executions from PostgreSQL.
3. The scheduler marks nodes `queued` and inserts `outbox_events` rows in the
   same transaction.
4. The scheduler publishes pending outbox events to a Redis Stream.
5. The scheduler marks the outbox event `published` after Redis accepts it.
6. Workers read Redis messages.
7. Workers acquire PostgreSQL leases before executing.
8. Workers heartbeat and complete using worker identity plus fencing token.

Worker leases include:

- Worker identity
- Node execution identity
- Lease expiration
- Attempt number
- Fencing token

Completion is idempotent for a repeated completion from the same fenced attempt.
Stale workers cannot overwrite newer attempts.

## Consequences

Positive:

- Queue messages may be duplicated without duplicating durable execution.
- Worker crashes can recover after lease expiration.
- Failed Redis publication does not lose work because the outbox remains
  durable.
- The design preserves PostgreSQL as the source of truth.

Negative:

- The system has more moving pieces: API, scheduler, worker, PostgreSQL, Redis.
- The current scheduler is intentionally simple and has only coarse backpressure.
- Redis pending-entry recovery is basic and will need more attention later.

## Alternatives Considered

Use Redis as the task source of truth:

- Rejected. It violates the project rule that PostgreSQL is authoritative.

Have the API publish directly to Redis:

- Rejected. A crash between database commit and Redis publish can lose dispatch.

Use Kubernetes Jobs for each node execution:

- Deferred. Jobs are valuable, but Phase 3 focuses on app-level leases,
  idempotency, and backpressure before Kubernetes worker pool design.

Claim exactly-once execution:

- Rejected. Phase 3 is explicitly at-least-once delivery with idempotent
  completion and fencing.
