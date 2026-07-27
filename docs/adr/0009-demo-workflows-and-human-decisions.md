# ADR 0009: Demo Workflows and Human Decisions

Date: 2026-07-27

## Status

Accepted

## Context

BuildPlane now has durable workflow execution, worker pools, an AI
classification service, a narrow operator, and a first observability slice.
The next learning step is to prove the platform with realistic synthetic
business workflows without introducing real external integrations or a full
workflow definition language.

The roadmap calls for invoice and freight exception workflows, reusable shared
components, a dependency graph, a human approval point, and guarded mock
external actions.

## Decision

Add two application-defined demo workflows:

- `demo.invoice-exception`
- `demo.freight-exception`

Both workflows use the same node graph:

```text
validate_demo_input -> classify_issue -> plan_demo_resolution -> await_human_approval -> record_mock_action -> compose_demo_summary
```

The approval node completes its node execution but marks the workflow run
`waiting_for_human` instead of creating the next node automatically.

Add a durable `human_decisions` table with a unique decision key per workflow
run. `POST /v1/workflow-runs/{id}/decisions` records either `approved` or
`rejected` decisions. An approved decision resumes the workflow by creating the
guarded mock action node. A rejected decision cancels the workflow.

Mock external actions are deterministic and synthetic. They require an approved
human decision in the worker lease context and do not call external systems.

## Consequences

- BuildPlane can now demonstrate human-in-the-loop workflow execution.
- The same execution platform handles deterministic nodes, AI classification,
  approval pauses, and guarded synthetic actions.
- Human decisions are idempotent and auditable.
- Workflow definitions remain compiled into Go code for now.
- The approval model is intentionally narrow and does not yet include role
  authorization, assignment queues, due dates, comments, or UI state.

## Alternatives Considered

Introduce a workflow DSL:

- Deferred. The platform still benefits more from explicit Go definitions while
  the execution model is being learned.

Treat approval as an automatic mock node:

- Rejected. That would skip the core human-in-the-loop behavior this phase is
  meant to teach.

Call real invoice, freight, ticketing, or payment systems:

- Rejected. External effects remain synthetic until authorization, secrets,
  provider contracts, and rollback behavior are designed.
