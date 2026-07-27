# Phase 11 Learning Notes: Frontend Operational UX

## What We Built

Phase 11 added a React and TypeScript operator console in `web/`.

The console supports:

- Workflow list and detail views.
- Synthetic invoice and freight workflow creation.
- Live workflow detail refresh through Server-Sent Events.
- Audit timeline inspection.
- Human approval and rejection controls.
- `issue_classifier` component version history.
- Affected workflow visibility.
- Candidate creation, evaluation, canary, promotion, and rollback actions.

## Kubernetes Lesson

A control plane is not only the reconciler or API. Human operators also need a
surface for seeing desired state, actual state, history, and safe actions.

In Kubernetes, `kubectl`, dashboards, controllers, and events all sit around
the same API-driven source of truth. BuildPlane follows that pattern:

```text
React console -> Go API -> PostgreSQL truth
```

The browser never talks directly to PostgreSQL, Redis, workers, or Kubernetes.
That boundary matters because the API owns validation, idempotency, audit, and
authorization decisions.

## Frontend State Lesson

The console keeps two kinds of state:

- Server state: workflow runs, audit records, component versions, dependencies.
- UI state: selected tab, selected run, selected version, form values, loading
  and message state.

The frontend should treat server state as cached data, not truth. Whenever an
operation completes, it reloads from the API.

## Realtime Lesson

Server-Sent Events are a one-way stream from server to browser. They are simpler
than WebSockets when the browser only needs to observe changing state.

For this phase, the API sends snapshots:

```text
workflow_run + audit_records
```

Snapshots are easy to reason about. If the browser misses an event, the next
snapshot still contains the current state.

## Operational UX Lesson

This console is not a marketing page. It is an operator surface:

- Dense enough to scan.
- Clear status labels.
- Actions near the state they affect.
- Audit history beside workflow input.
- Release controls beside component version state.

Good operational UX reduces uncertainty. The operator should see what happened,
what is pending, and what action is safe to take next.

## Commands

Run the frontend locally:

```bash
cd web
npm install
npm run dev
```

Run frontend checks:

```bash
cd web
npm run typecheck
npm test
npm run build
```

With the Docker Compose stack running, Vite proxies `/api` to
`http://localhost:8080`.
