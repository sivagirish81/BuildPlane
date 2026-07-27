# Phase 04: Local End-to-End Workflow

## Concepts Learned

### Workflow Orchestration

Workflow orchestration coordinates multiple steps, persists their state, and
decides what should run next.

It exists because real work is not one function call. Steps can fail, retry,
pause, resume, and finish at different times.

In BuildPlane, workflow creation creates the first pending node. When a worker
successfully completes a node, PostgreSQL creates the next pending node in the
same transaction. When the terminal node succeeds, the workflow run succeeds.

Common mistake: keeping the workflow plan only in memory. If the process dies,
the database must still explain what happened.

Inspect or debug it:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select id, workflow_name, status from workflow_runs;'
```

### Node State Machine

A node state machine defines allowed node execution states and transitions.

Phase 4 uses:

```text
pending -> queued -> running -> succeeded
pending -> queued -> running -> pending
pending -> queued -> running -> failed
```

The second path is retry. The third path is exhausted failure.

Common mistake: writing arbitrary statuses from many places. BuildPlane keeps
state changes inside repository methods.

Inspect or debug it:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select node_name, status, attempt, error from node_executions order by created_at;'
```

### Deterministic Code Component

A deterministic code component is workflow logic implemented by normal code.

It exists for steps that must be repeatable, testable, and safe. Validation,
authorization, arithmetic, and policy checks should not depend on an LLM.

In Phase 4, `validate_input` ensures `case_id` exists, and `compose_summary`
returns a synthetic summary.

Common mistake: using an LLM for a deterministic validation rule.

Inspect or debug it:

```bash
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod \
  go test ./services/control-plane/internal/workflows
```

### Audit Trail

An audit trail is durable product history, not just process logs.

It exists so operators can answer what happened to a workflow run and which
component made each state transition.

BuildPlane records audit rows for workflow creation, node creation, node
queueing, node running, node success, retry scheduling, failure, and workflow
success/failure.

Common mistake: assuming logs are enough. Logs can rotate or be sampled; audit
records are part of the durable application model.

Inspect or debug it:

```bash
curl -i http://localhost:8080/v1/workflow-runs/<workflow-run-id>/audit
```

Or directly:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select event_type, actor_type, actor_id, created_at from audit_records order by id;'
```

### Retry Semantics

Retry semantics define when work should be attempted again.

It exists because some failures are transient, but infinite retries can create
storms and hide broken logic.

In Phase 4, each node has `MaxAttempts`. A failed attempt below that number goes
back to `pending`. Once attempts are exhausted, the node and workflow fail.

Common mistake: retrying forever without recording the reason and attempt.

Inspect or debug it:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select node_name, status, attempt, error from node_executions;'
```

## Architecture Diagram

```mermaid
flowchart TD
  API["POST /v1/workflow-runs"] --> RunTx["Create workflow + first node + audit"]
  Scheduler["Scheduler"] --> QueueTx["pending -> queued + outbox + audit"]
  QueueTx --> Redis["Redis Stream"]
  Worker["Worker"] --> Lease["Acquire lease + audit"]
  Lease --> Exec["Execute deterministic node"]
  Exec --> Success["Complete node"]
  Exec --> Failure["Fail or retry node"]
  Success --> Next["Create next pending node"]
  Success --> Done["Mark workflow succeeded"]
  Failure --> Retry["running -> pending"]
  Failure --> Failed["Mark workflow failed"]
  Next --> Audit["audit_records"]
  Done --> Audit
  Retry --> Audit
  Failed --> Audit
```

## Important Code Paths

- `services/control-plane/internal/workflows/definitions.go`
  - Defines `phase4.local-demo`.
  - Executes `validate_input` and `compose_summary`.

- `services/control-plane/internal/workflows/execution.go`
  - Worker orchestration calls deterministic node execution.
  - Worker passes next node and max attempts to the repository.

- `services/control-plane/internal/postgres/execution_repository.go`
  - Persists node success, next-node creation, retry, and workflow terminal
    state.

- `services/control-plane/internal/postgres/audit.go`
  - Inserts audit records inside existing transactions.

- `services/control-plane/internal/httpapi/server.go`
  - Adds `GET /v1/workflow-runs/{id}/audit`.

## Commands

Run tests:

```bash
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod \
  go test ./services/control-plane/...
```

Start dependencies:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml up -d
export BUILDPLANE_DATABASE_URL='postgres://buildplane:buildplane_dev_password@localhost:5432/buildplane?sslmode=disable'
export BUILDPLANE_REDIS_URL='redis://localhost:6379/0'
```

Run API, scheduler, and worker in separate terminals:

```bash
go run ./services/control-plane/cmd/buildplane-control-plane
go run ./services/control-plane/cmd/buildplane-scheduler
BUILDPLANE_WORKER_ID=local-worker-1 go run ./services/control-plane/cmd/buildplane-worker
```

Create a successful workflow:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: phase4-success-001' \
  -d '{"workflow_name":"phase4.local-demo","input":{"case_id":"synthetic-case-001"}}'
```

Create a workflow that retries and eventually fails:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: phase4-failure-001' \
  -d '{"workflow_name":"phase4.local-demo","input":{}}'
```

Inspect audit:

```bash
curl -i http://localhost:8080/v1/workflow-runs/<workflow-run-id>/audit
```

## Debugging Exercises

### Break: missing input

Create `phase4.local-demo` with `{}` as input.

Expected result: `validate_input` fails, retries once, then the workflow fails.

Recover: create a new workflow run with a new `Idempotency-Key` and a valid
`case_id`.

### Break: stale worker

Let worker A acquire a lease, wait for the lease to expire, then let worker B
take over. Worker A should not be able to heartbeat or complete with the old
fencing token.

Expected result: stale operations fail with a stale lease error.

### Break: duplicate message

Publish a duplicate Redis message for a node that already succeeded.

Expected result: PostgreSQL lease/completion rules prevent duplicate durable
completion.

## Tradeoffs

The workflow definition is application code. That is intentionally simple and
testable. It is not yet a general workflow definition system.

Creating the next node only after successful completion makes sequential
behavior easy to understand. Later, parallel branches will need an explicit
dependency graph.

The audit table stores structured JSON details, but Phase 4 keeps those details
small and synthetic.

## Five Interview Questions

1. Why create the next node inside the same transaction as node completion?
2. Why is an audit trail different from logs?
3. What transition represents a retry?
4. Why should validation be deterministic?
5. How does the worker know what node to run?

## Five Short Answers

1. So the system never records a completed node without also recording what
   should happen next.
2. Audit is durable application history; logs are operational telemetry.
3. `running -> pending`.
4. Deterministic validation is repeatable and enforceable.
5. It uses the leased workflow name and node name to find an application-defined
   node runner.

## Teach It Back Exercise

Explain this full path in your own words:

```text
workflow created -> validate_input -> compose_summary -> workflow succeeded
```

Then explain where PostgreSQL transactions protect that path.
