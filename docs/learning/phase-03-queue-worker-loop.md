# Phase 03: Queue and Worker Loop

## Concepts Learned

### At-Least-Once Delivery

At-least-once delivery means a task can be delivered one or more times.

It exists because distributed systems lose acknowledgments, restart processes,
and retry operations. A scheduler might publish to Redis and crash before
recording success. A worker might complete work and crash before acknowledging a
message.

BuildPlane handles this by making PostgreSQL authoritative. Redis delivery is a
hint; the database lease decides whether execution is allowed.

Common mistake: saying "Redis Streams means exactly once." Redis Streams can
support reliable delivery patterns, but application state still has to handle
duplicates.

Inspect or debug it:

```bash
redis-cli XREAD COUNT 10 STREAMS buildplane:node-executions 0
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select id, status, attempt, fencing_token from node_executions;'
```

### Transactional Outbox

A transactional outbox is a database table of events that need to be published.
The event is written in the same transaction as the state change.

It exists because PostgreSQL and Redis do not share a transaction. BuildPlane
cannot atomically commit PostgreSQL state and Redis publication as one operation.

In Phase 3, the scheduler marks a node `queued` and inserts an `outbox_events`
row in one transaction. It then publishes the outbox event to Redis and marks it
`published`.

Common mistake: publishing directly to Redis from the API after inserting a row.
If the process crashes between those steps, durable work exists with no dispatch
message.

Inspect or debug it:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select id, topic, status, attempts, last_error from outbox_events order by id;'
```

### Worker Lease

A worker lease is a temporary database claim over a node execution.

It exists because workers can crash. A lease lets the system recover work after
expiration.

BuildPlane records:

- Worker identity
- Node execution identity
- Lease expiration
- Attempt number
- Fencing token

Common mistake: treating a Redis message as ownership. The Redis message only
tells a worker what to attempt. PostgreSQL grants ownership.

Inspect or debug it:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select id, status, lease_worker_id, lease_expires_at, attempt from node_executions;'
```

### Heartbeat

A heartbeat extends a lease while the worker is alive.

It exists so long-running work is not reclaimed prematurely.

BuildPlane heartbeats require the node ID, worker ID, and fencing token. If a
new worker has taken over, the stale heartbeat updates zero rows and fails.

Common mistake: heartbeating by node ID only. That allows stale workers to keep
work alive after takeover.

Inspect or debug it:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select id, lease_expires_at, updated_at from node_executions;'
```

### Fencing Token

A fencing token is a monotonically increasing number assigned when a lease is
acquired.

It exists to block stale workers. If worker A pauses, its lease expires, worker
B takes over, and worker A wakes up, worker A still has the old token.

BuildPlane completion and heartbeat require the current token.

Common mistake: checking only worker ID. Fencing tokens protect attempts, not
just workers.

Inspect or debug it:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select id, attempt, fencing_token, status from node_executions;'
```

## Architecture Diagram

```mermaid
flowchart TD
  Create["POST /v1/workflow-runs"] --> Tx1["DB transaction"]
  Tx1 --> Run["workflow_runs: queued"]
  Tx1 --> Node["node_executions: pending"]
  Scheduler["buildplane-scheduler"] --> Tx2["claim schedulable nodes"]
  Tx2 --> Queued["node_executions: queued"]
  Tx2 --> Outbox["outbox_events: pending"]
  Scheduler --> Publish["publish to Redis Stream"]
  Publish --> Published["outbox_events: published"]
  Worker["buildplane-worker"] --> Read["XREADGROUP"]
  Read --> Lease["Acquire DB lease"]
  Lease --> Running["node_executions: running"]
  Running --> Heartbeat["Heartbeat with fencing token"]
  Heartbeat --> Complete["Complete with fencing token"]
  Complete --> Succeeded["workflow_runs/node_executions: succeeded"]
```

## Important Code Paths

- `migrations/0002_queue_worker_loop.sql`
  - Creates `node_executions` and `outbox_events`.

- `services/control-plane/internal/postgres/workflow_repository.go`
  - Workflow creation now inserts the initial pending node execution.

- `services/control-plane/internal/postgres/execution_repository.go`
  - Claims schedulable nodes.
  - Publishes outbox state transitions.
  - Acquires leases.
  - Heartbeats.
  - Completes or fails nodes with fencing.

- `services/control-plane/internal/workflows/execution.go`
  - Contains scheduler and worker orchestration logic.

- `services/control-plane/internal/queue/redis.go`
  - Publishes and consumes Redis Stream messages.

- `services/control-plane/cmd/buildplane-scheduler/main.go`
  - Runs the scheduler loop.

- `services/control-plane/cmd/buildplane-worker/main.go`
  - Runs the worker loop.

## Commands

Run unit tests:

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

Run API:

```bash
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod \
  go run ./services/control-plane/cmd/buildplane-control-plane
```

Run scheduler:

```bash
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod \
  go run ./services/control-plane/cmd/buildplane-scheduler
```

Run worker:

```bash
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod \
  BUILDPLANE_WORKER_ID=local-worker-1 \
  go run ./services/control-plane/cmd/buildplane-worker
```

Create work:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: phase3-demo-001' \
  -d '{"workflow_name":"phase3-demo","input":{"case_id":"synthetic-case-001"}}'
```

Inspect state:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select id, status, attempt, lease_worker_id, fencing_token from node_executions;'
```

Inspect queue:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec redis \
  redis-cli XINFO STREAM buildplane:node-executions
```

## Debugging Exercises

### Break: Redis unavailable

Start PostgreSQL but stop Redis. Run the scheduler.

Expected result: outbox events remain `failed` or `pending`, and `last_error`
records the publish problem.

Recover: start Redis and rerun the scheduler.

### Break: worker crash

Create a workflow run, let the worker acquire a lease, then stop the worker
before completion. After the lease expires, run another worker.

Expected result: a later worker can acquire the task with a newer fencing token.

Recover: run the worker again and inspect `node_executions`.

### Break: duplicate queue message

Manually publish a duplicate Redis Stream message for the same node execution.

Expected result: PostgreSQL prevents duplicate execution if the node already
succeeded or is leased.

Recover: acknowledge or drain duplicate messages after confirming durable state.

## Tradeoffs

The scheduler is intentionally simple. It claims a small batch, publishes
outbox events, and retries failed publication after a short delay. Later phases
will need richer backpressure based on tenant limits, queue depth, oldest task
age, retry storms, and downstream capacity.

The worker currently runs a placeholder deterministic node. Phase 4 will replace
that with a clearer local workflow definition and node state machine.

Redis pending-entry recovery is basic. Future worker logic should inspect and
reclaim stuck Redis consumer-group pending entries.

## Five Interview Questions

1. Why does BuildPlane need a transactional outbox?
2. Why is Redis not the source of truth?
3. What problem does a worker lease solve?
4. What problem does a fencing token solve?
5. Why is at-least-once delivery acceptable here?

## Five Short Answers

1. To durably remember events that must be published after database state
   changes.
2. PostgreSQL holds durable workflow state; Redis accelerates dispatch.
3. It lets the system recover work after worker crashes.
4. It prevents stale workers from overwriting newer attempts.
5. Because duplicate messages are handled by idempotent database transitions.

## Teach It Back Exercise

Explain this failure:

```text
worker A gets a task
worker A pauses
lease expires
worker B gets the task
worker A wakes up and tries to complete
```

Then explain exactly how `lease_expires_at` and `fencing_token` protect the
workflow state.
