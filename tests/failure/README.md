# Failure Scenarios

Manual failure checks for the MVP:

- Delete a runner pod and wait for lease expiration plus retry.
- Restart Redis and verify reconciliation re-enqueues `QUEUED` jobs.
- Send a duplicate completion callback and verify it is rejected.
- Cancel a run and verify the Kubernetes Job is deleted.
- Stop MinIO and verify cache misses do not fail user commands.
