# ADR 0011: Frontend Operational Console

Date: 2026-07-27

## Status

Accepted

## Context

BuildPlane now has durable workflow runs, audit records, human decisions, demo
workflows, and component release gates. Operators need a UI that shows this
state without bypassing the API or inventing a second source of truth.

The UI should teach real operational product design:

- Read durable workflow state from the control-plane API.
- Show audit records beside the current workflow state.
- Submit human approvals through the existing idempotent decision endpoint.
- Show reusable component versions, release status, and affected workflows.
- Use a simple live-update mechanism before introducing a heavier realtime
  platform.

## Decision

Add a Vite, React, and TypeScript app under `web/`.

The frontend uses the Go control-plane API as its only backend. It does not
connect directly to PostgreSQL, Redis, or Kubernetes.

Add small read endpoints required by the UI:

- `GET /v1/workflow-runs?limit=50`
- `GET /v1/workflow-runs/{id}/events`
- `GET /v1/component-versions?component_name=issue_classifier&limit=25`

Use Server-Sent Events for workflow detail updates. The event payload is a
snapshot containing the workflow run and its audit records. This is enough for
the current operational view and keeps reconnection/replay behavior simple.

Add a frontend CI job that runs on pull requests to `main` and pushes to
`main`:

- `npm ci`
- `npm run typecheck`
- `npm test`
- `npm run build`

## Consequences

- Operators can create demo runs, inspect workflow details, review audit
  records, submit human decisions, and operate synthetic component releases
  from one console.
- The frontend remains stateless. Refreshing the browser reloads truth from the
  API.
- SSE is intentionally narrow. It streams workflow snapshots only; broader
  event fanout, authorization, assignments, notifications, and multi-tenant UI
  concerns are deferred.
- Vite proxies `/api` to `localhost:8080` for local development.
