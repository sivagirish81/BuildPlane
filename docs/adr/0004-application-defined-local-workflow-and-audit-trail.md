# ADR 0004: Application-Defined Local Workflow and Audit Trail

Date: 2026-07-26

## Status

Accepted

## Context

BuildPlane can now create durable workflow runs, schedule pending node
executions, publish queue messages through Redis Streams, acquire worker leases,
heartbeat, and complete placeholder work.

Phase 4 needs a real end-to-end workflow without introducing a custom workflow
language, LLM service, or external integrations too early.

## Decision

BuildPlane will implement a small deterministic workflow in Go application code.

The workflow is named `phase4.local-demo` and has two nodes:

1. `validate_input`
2. `compose_summary`

The worker resolves the workflow definition by `workflow_name`, executes the
leased node, and returns:

- Node result JSON
- Next node name, if any
- Max attempts for retry decisions

PostgreSQL completion logic persists the current node result and either:

- Creates the next pending node execution in the same transaction, or
- Marks the workflow run `succeeded` when the terminal node completes.

BuildPlane also adds durable `audit_records`. State-changing repository methods
write audit records in the same transaction as the state transition.

## Consequences

Positive:

- BuildPlane now has a complete local request-to-completion path.
- Workflow ordering is explicit and testable.
- Audit history is durable and queryable.
- Retry behavior is bounded per node.

Negative:

- Workflow definitions are compiled into Go code for now.
- The local workflow is intentionally simple and not yet a general workflow
  model.
- Live integration testing still depends on Docker availability.

## Alternatives Considered

Introduce a custom workflow language:

- Rejected. It would add parser/schema complexity before the execution model is
  mature.

Store every node at workflow creation:

- Rejected for now. Creating the next node at completion makes sequential
  execution easier to inspect.

Use an LLM node:

- Deferred to Phase 5. Phase 4 is deliberately deterministic.
