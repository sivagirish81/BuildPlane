# ADR 0002: PostgreSQL-Backed Idempotent Workflow Creation

Date: 2026-07-26

## Status

Accepted

## Context

BuildPlane needs durable workflow state before it can safely dispatch work to
queues or Kubernetes workers. Clients may retry requests after network failures,
timeouts, or lost responses. Retried creation requests must not create duplicate
workflow runs.

## Decision

Workflow runs are stored in PostgreSQL in a `workflow_runs` table. The first
state is `queued`.

`POST /v1/workflow-runs` requires an `Idempotency-Key` header. BuildPlane stores
the idempotency key with a SHA-256 request hash derived from the workflow name
and canonical JSON input.

Creation runs inside a PostgreSQL transaction:

1. Attempt to insert the workflow run.
2. If the idempotency key is new, commit and return `201 Created`.
3. If the key already exists, load the existing row.
4. If the stored request hash matches, commit and return the existing run with
   `200 OK`.
5. If the stored request hash differs, return `409 Conflict`.

Schema changes are applied through versioned SQL files in `migrations/`, with
applied files recorded in `schema_migrations`.

The Go service uses `database/sql` with the pgx PostgreSQL driver.

## Consequences

Positive:

- PostgreSQL becomes the durable source of truth for workflow runs.
- Duplicate workflow creation is prevented by a database uniqueness constraint.
- Idempotency behavior is safe across process restarts.
- The request hash catches accidental idempotency key reuse with different
  payloads.

Negative:

- The service now requires PostgreSQL before it can start.
- Integration tests require a running database, which currently depends on
  Docker availability.
- The migration runner is intentionally simple and may need replacement or
  hardening before complex schema evolution.

## Alternatives Considered

Use in-memory idempotency:

- Rejected. It would fail across restarts and replicas.

Use Redis as the idempotency store:

- Rejected for workflow creation because PostgreSQL is the durable source of
  truth.

Use an ORM:

- Rejected for now. Explicit SQL is clearer while the schema is small.

Use `golang-migrate` immediately:

- Rejected for now. A tiny migration runner teaches the migration concept
  without another dependency. A mature migration tool can be adopted when schema
  complexity justifies it.
