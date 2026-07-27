# Phase 02: Durable Control Plane Skeleton

## Concepts Learned

### REST API Boundary

A REST API boundary is the contract between clients and BuildPlane. Clients send
HTTP requests with JSON bodies; BuildPlane validates them, applies system rules,
persists state, and returns JSON responses.

It exists so clients do not need to know about internal tables, queues, workers,
or Kubernetes objects.

In this phase:

- `POST /v1/workflow-runs` creates a durable workflow run.
- `GET /v1/workflow-runs/{id}` reads a workflow run.
- `/healthz`, `/readyz`, and `/version` remain operational endpoints.

Common mistake: putting database logic directly in handlers. BuildPlane keeps a
workflow service and repository between HTTP and SQL.

Inspect or debug it:

```bash
curl -i http://localhost:8080/version
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-001' \
  -d '{"workflow_name":"phase4.local-demo","input":{"case_id":"synthetic-case-001"}}'
```

### PostgreSQL as Source of Truth

PostgreSQL is the durable record of workflow state. If the API Pod restarts,
future requests still see the workflow run because it lives in the database.

It exists because workflow orchestration cannot rely on memory, Pod lifetime, or
queue messages alone.

In BuildPlane, `workflow_runs` stores:

- Workflow run ID
- Workflow name
- Status
- Input JSON
- Idempotency key
- Request hash
- Correlation ID
- Creation and update timestamps

Common mistake: treating Redis or a queue as the only record of work. Queues are
for dispatch; PostgreSQL is the durable ledger.

Inspect or debug it:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select id, workflow_name, status, created_at from workflow_runs;'
```

### Migration

A migration is a versioned database change. It lets a team create and evolve the
schema predictably.

It exists because production databases cannot be rebuilt from scratch every time
the application changes.

In BuildPlane, startup runs SQL files from `migrations/` and records applied
filenames in `schema_migrations`.

Common mistake: manually changing a local database and forgetting to commit the
schema change.

Inspect or debug it:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select version, applied_at from schema_migrations;'
```

### Idempotency

Idempotency means retrying the same logical request does not create duplicate
side effects.

It exists because clients can lose responses. The server may commit the
workflow, then the network fails, and the client retries.

BuildPlane requires `Idempotency-Key` on workflow creation. It stores both the
key and a request hash:

- Same key and same request: return the original run.
- Same key and different request: return `409 Conflict`.
- New key: create a new run.

Common mistake: storing only the key. Without the request hash, two different
requests can accidentally share one key.

Inspect or debug it:

```bash
# First request should return 201.
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-001' \
  -d '{"workflow_name":"phase4.local-demo","input":{"case_id":"synthetic-case-001"}}'

# Same request should return 200 and replayed true.
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-001' \
  -d '{"workflow_name":"phase4.local-demo","input":{"case_id":"synthetic-case-001"}}'

# Different request with same key should return 409.
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-001' \
  -d '{"workflow_name":"phase4.local-demo","input":{"case_id":"synthetic-case-002"}}'
```

### Transaction Boundary

A transaction boundary defines which database operations succeed or fail
together.

It exists because the idempotency check and workflow creation must be atomic.

In BuildPlane, `CreateRun` starts a transaction, attempts the insert, checks an
existing row if the idempotency key already exists, and commits only after the
correct result is known.

Common mistake: doing "check if key exists" and "insert row" as separate
non-transactional steps. Two concurrent requests can both think they should
create the run.

Inspect or debug it:

```sql
\d workflow_runs
select idempotency_key, request_hash from workflow_runs;
```

### Correlation ID

A correlation ID is a value that follows one request through logs and responses.

It exists so distributed debugging has a thread to pull. Later, the same ID will
span API, worker, AI service, and frontend events.

In this phase, the API accepts `X-Correlation-ID` or generates one. It echoes it
in response headers and structured logs.

Common mistake: logging lots of details but no stable request identifier.

Inspect or debug it:

```bash
curl -i http://localhost:8080/version -H 'X-Correlation-ID: debug-001'
```

Then look for `correlation_id=debug-001` or its JSON equivalent in logs.

## Architecture Diagram

```mermaid
flowchart TD
  Client["Client"] --> Handler["HTTP handler"]
  Handler --> Corr["Correlation ID middleware"]
  Handler --> Validate["Validate request"]
  Validate --> Service["Workflow service"]
  Service --> Hash["Canonical JSON + request hash"]
  Hash --> Repo["PostgreSQL repository"]
  Repo --> Tx["Transaction"]
  Tx --> Runs["workflow_runs"]
  Startup["Process startup"] --> Migrator["Migration runner"]
  Migrator --> Schema["schema_migrations"]
  Migrator --> Runs
```

## Important Code Paths

- `services/control-plane/cmd/buildplane-control-plane/main.go`
  - Reads `BUILDPLANE_DATABASE_URL`.
  - Opens PostgreSQL.
  - Applies migrations.
  - Wires the workflow service into the HTTP server.

- `services/control-plane/internal/httpapi/server.go`
  - Handles workflow creation and lookup.
  - Adds correlation IDs.
  - Converts domain errors into HTTP status codes.

- `services/control-plane/internal/workflows/workflows.go`
  - Validates workflow creation.
  - Canonicalizes JSON input.
  - Computes request hashes.
  - Defines workflow statuses and domain errors.

- `services/control-plane/internal/postgres/workflow_repository.go`
  - Inserts workflow runs transactionally.
  - Replays existing idempotent requests.
  - Detects idempotency conflicts.

- `services/control-plane/internal/postgres/migrate.go`
  - Applies SQL migration files once.

- `migrations/0001_create_workflow_runs.sql`
  - Creates the durable `workflow_runs` schema.

## Commands

Run unit tests:

```bash
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod \
  go test ./services/control-plane/...
```

Start PostgreSQL:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml up -d
```

Run the control plane:

```bash
export BUILDPLANE_DATABASE_URL='postgres://buildplane:buildplane_dev_password@localhost:5432/buildplane?sslmode=disable'
env GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod \
  go run ./services/control-plane/cmd/buildplane-control-plane
```

Create a workflow:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-001' \
  -H 'X-Correlation-ID: debug-001' \
  -d '{"workflow_name":"phase4.local-demo","input":{"case_id":"synthetic-case-001"}}'
```

Read a workflow:

```bash
curl -i http://localhost:8080/v1/workflow-runs/<workflow-run-id>
```

Inspect state:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select id, workflow_name, status, idempotency_key, created_at from workflow_runs;'
```

## Debugging Exercises

### Break: missing database URL

Run the service without `BUILDPLANE_DATABASE_URL`.

Expected result: startup fails with `BUILDPLANE_DATABASE_URL is required`.

Recover: export a valid database URL and rerun.

### Break: database unavailable

Stop PostgreSQL:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml down
```

Run the service.

Expected result: startup fails while pinging PostgreSQL.

Recover: start PostgreSQL again and rerun the service.

### Break: idempotency conflict

Reuse an `Idempotency-Key` with a different `input`.

Expected result: `409 Conflict`.

Recover: use a new `Idempotency-Key` for a different logical request.

## Tradeoffs

The migration runner is small and explicit. That is useful for learning, but it
is not a full migration product. Later, if schema changes need rollbacks,
locking, checksums, or richer operational controls, BuildPlane can adopt a
dedicated migration tool.

The API uses Go's standard library. That keeps request handling transparent
while the API surface is tiny.

The request hash uses canonical JSON input. This means formatting differences do
not create idempotency conflicts, but semantic differences do.

## Five Interview Questions

1. Why does workflow creation need an idempotency key?
2. Why store a request hash with the idempotency key?
3. Why should PostgreSQL be the source of truth instead of Redis?
4. What database constraint prevents duplicate idempotency keys?
5. What belongs inside the transaction boundary for workflow creation?

## Five Short Answers

1. To prevent duplicate workflow runs when clients retry after uncertain
   failures.
2. To detect the same key being reused with a different request body.
3. PostgreSQL provides durable relational state; Redis is better used for
   dispatch and short-lived coordination.
4. A unique constraint on `workflow_runs.idempotency_key`.
5. Insert the run, load an existing run on conflict, compare request hashes, and
   commit the chosen result.

## Teach It Back Exercise

Explain why this sequence is dangerous:

```text
select idempotency key
if not found, insert workflow run
```

Then explain how a unique constraint plus a transaction makes it safer.
