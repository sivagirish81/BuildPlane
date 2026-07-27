# Phase 9 Learning Artifact: Demo Workflows

## What You Built

Phase 9 adds two synthetic operational workflows:

- `demo.invoice-exception`
- `demo.freight-exception`

Both use a shared graph:

```text
validate_demo_input -> classify_issue -> plan_demo_resolution -> await_human_approval -> record_mock_action -> compose_demo_summary
```

The important new behavior is the approval pause. `await_human_approval`
finishes its own node execution, but the workflow run moves to
`waiting_for_human`. The next node is created only after a human decision is
submitted through the API.

## What To Inspect

Start a demo run:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: invoice-demo-001' \
  --data @examples/demo-workflows/invoice-exception.json
```

When the run reaches the approval point, approve it:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs/<workflow-run-id>/decisions \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: invoice-demo-approval-001' \
  -d '{"decision":"approved","actor_id":"operator-1","reason":"Synthetic approval for the learning demo"}'
```

Rejecting uses the same endpoint:

```bash
curl -i -X POST http://localhost:8080/v1/workflow-runs/<workflow-run-id>/decisions \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: invoice-demo-rejection-001' \
  -d '{"decision":"rejected","actor_id":"operator-1","reason":"Synthetic rejection for the learning demo"}'
```

Then inspect state:

```bash
curl -i http://localhost:8080/v1/workflow-runs/<workflow-run-id>
curl -i http://localhost:8080/v1/workflow-runs/<workflow-run-id>/audit
```

Database queries worth running:

```bash
docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select id, workflow_name, status, updated_at from workflow_runs order by created_at desc limit 5;'

docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select workflow_run_id, node_name, status, worker_pool, result from node_executions order by created_at;'

docker compose -f deploy/docker/docker-compose.postgres.yaml exec postgres \
  psql -U buildplane -d buildplane \
  -c 'select workflow_run_id, decision_key, decision, actor_id, reason from human_decisions order by created_at;'
```

## Mental Model

The worker is allowed to execute a node only after it gets a PostgreSQL lease.
The approval node uses that same worker path, but it asks PostgreSQL to pause
the workflow instead of creating the next node.

The human decision endpoint is the resume boundary:

```text
waiting_for_human + approved decision -> queued + record_mock_action
waiting_for_human + rejected decision -> canceled
```

The guarded mock action does not trust that it appeared in the graph. It also
checks that the worker lease includes an approved human decision.

## Common Debugging Moves

- If nothing runs, inspect `node_executions.status` and scheduler logs.
- If a run is stuck at approval, check `workflow_runs.status`.
- If approval fails, confirm the run is `waiting_for_human` and the request has
  an `Idempotency-Key`.
- If the mock action fails, inspect whether `human_decisions` contains an
  approved row for the run.
