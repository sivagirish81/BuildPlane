# BuildPlane implementation plan

## Assumptions

- BuildPlane is an interview-ready MVP, not a hosted multi-tenant CI product.
- PostgreSQL is the durable source of truth. Redis is a derived ready queue and may be rebuilt from PostgreSQL.
- Kubernetes Jobs are the production execution primitive. The local demo can run the same runner image in a kind cluster.
- MVP repositories are public or local fixtures. Private Git credentials, tenant secrets, and workload identity are documented production work.
- Commands are stored as arrays but executed through a shell in the runner because CI users expect shell syntax. The security tradeoff is explicit in the docs.
- Logs are stored in PostgreSQL for simplicity. Production should move log chunks to object storage.
- Cache archives use S3-compatible object storage and content-addressed keys. The MVP uses gzip when zstd tooling is unavailable, while preserving the `.tar.zst` object layout contract for compatibility documentation.

## Milestone 1: vertical execution path

- Create database schema and migrations for tenants, workflows, workflow runs, jobs, attempts, logs, artifacts, and cache metadata.
- Implement API endpoints for workflow creation, run creation, run read, job logs, cancellation, health, metrics, and internal runner callbacks.
- Persist workflow runs and jobs in one transaction and enqueue runnable jobs.
- Implement scheduler polling, atomic job claims, attempt creation, lease token hashing, Kubernetes Job creation, and basic reconciliation.
- Implement runner command execution, log upload, heartbeat renewal, cache restore/upload hooks, and completion callback.
- Provide local Dockerfiles, Docker Compose infra, kind manifests, and a sample workflow.

## Milestone 2: durable queue and state machine

- Redis priority queues are derived from jobs in `QUEUED` state.
- Queue reconciliation periodically re-enqueues missing jobs.
- Duplicate deliveries are ignored by conditional PostgreSQL state transitions.
- Domain state transitions are centralized and covered by unit tests.

## Milestone 3: scheduling

- Priority levels use weights `critical=8`, `high=4`, `normal=2`, `low=1`.
- Tenant fairness uses deficit round-robin and job cost derived from requested CPU.
- Admission control enforces global, tenant, and resource limits before scheduling.
- Canary scheduler mode receives the same candidate snapshot and records stable/canary disagreements.

## Milestone 4: reliability

- Each attempt owns a lease token; only the hash is stored.
- Runners must renew leases and completion is rejected for expired, stale, or superseded attempts.
- Reconciliation marks expired attempts as lost and requeues retryable jobs with exponential backoff.
- Cancellation terminates queued and active jobs and asks Kubernetes to delete active Jobs.

## Milestone 5: cache and autoscaling

- Cache keys are SHA-256 hashes of normalized prefix, key file contents, commands, runner version, OS, architecture, and selected environment.
- Safe archive extraction rejects path traversal.
- Autoscaling decisions use queue compute seconds, old job age, pending pods, cooldowns, min/max, and step limits.

## Milestone 6: operations

- Services emit JSON logs with bounded labels and no lease tokens.
- Prometheus metrics cover API, queue, scheduler, execution, leases, cache, autoscaling, and reconciliation.
- Grafana dashboards are provisioned for queue health, scheduling, reliability, cache, and autoscaling.

## Milestone 7: demonstration and documentation

- `make demo` builds the runner image, starts local infra, deploys to kind, submits a sample workflow, waits for completion, prints logs, and exercises idempotency.
- Documentation explains architecture, consistency, scheduling, failure recovery, caching, autoscaling, and interview talking points.

## Current limitations to verify after implementation

- PostgreSQL-backed logs are bounded by local demo use, not indefinite CI retention.
- Kubernetes startup latency means this MVP is best for jobs longer than a few seconds.
- Shared kind nodes are not a secure boundary for hostile untrusted code.
- Redis loss can temporarily delay execution until queue reconciliation runs.
- Lease expiration can create concurrent attempts; stale-result protection preserves durable correctness but cannot prevent duplicate side effects inside user commands.
