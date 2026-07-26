# Consistency

BuildPlane does not claim exactly-once execution. It provides at-least-once delivery with idempotent state transitions.

Strongly consistent operations:

- Workflow and run creation inside PostgreSQL transactions.
- Idempotency by unique tenant/idempotency-key.
- Job claims with status and version predicates.
- Runner callbacks validated by latest attempt and lease.

Eventually consistent operations:

- Redis queue membership.
- Kubernetes Job state versus database state.
- Metrics and dashboards.
- Cache metadata versus object storage during transient failures.

Redis and PostgreSQL can temporarily disagree. PostgreSQL wins. Reconciliation repairs Redis and marks expired attempts. Duplicate work is possible when a lease expires while an old runner continues, so user commands should be idempotent or isolated.
