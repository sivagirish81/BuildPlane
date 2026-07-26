# State Machine

Job states:

- `QUEUED`: persisted and runnable.
- `SCHEDULING`: scheduler claimed the job with a conditional update.
- `SCHEDULED`: an attempt exists and a Kubernetes Job should exist.
- `RUNNING`: runner accepted the attempt and started.
- `SUCCEEDED`: user commands completed successfully.
- `FAILED`: terminal failure or exhausted retries.
- `TIMED_OUT`: deadline exceeded.
- `CANCELLED`: user cancellation won.
- `LOST`: the attempt lease expired or the worker disappeared.

Allowed transitions are implemented in `internal/domain/status.go`. Duplicate transitions are valid only when source and destination are identical; terminal states do not transition to other terminal states.

State updates use conditional SQL. Duplicate queue deliveries and stale runner callbacks fail harmlessly because the expected status, version, active attempt, and lease must match.
