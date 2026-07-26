# Failure Model

API crash after database commit but before Redis enqueue: PostgreSQL has `QUEUED` jobs. Scheduler reconciliation re-enqueues them.

Redis restart: ready queues are lost. PostgreSQL remains correct. Reconciliation finds `QUEUED` jobs and rebuilds Redis.

Duplicate queue delivery: scheduler attempts a conditional claim. Only one update from `QUEUED` with the expected version succeeds.

Scheduler crash after claim: the job can be left in `SCHEDULING` or `SCHEDULED`. The reconciler is responsible for returning stale work to `QUEUED` or expiring the attempt.

Kubernetes API failure: BuildPlane does not hold a database transaction while calling Kubernetes. The MVP logs the error; production should move the job back to `QUEUED` with retry classification.

Runner pod deletion: heartbeats stop. Lease expiration marks the attempt `LOST` and requeues the job if attempts remain.

Duplicate completion callback: the first valid completion wins. Later callbacks fail because the attempt is no longer active.

Expired worker reports success: the API rejects it because the lease expired or a newer attempt exists.

Cancellation: queued and active jobs are marked `CANCELLED`; active Kubernetes Jobs are deleted.

MinIO unavailable during cache restore: cache miss/restore failure must not fail the job. Upload failures are recoverable infrastructure errors in production.
