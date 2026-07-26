# Architecture

BuildPlane has four deployable services. The API accepts workflow definitions and run submissions. PostgreSQL stores tenants, workflows, runs, jobs, attempts, cache metadata, artifacts, and logs. Redis stores derived ready queues by priority and tenant. The scheduler reads Redis candidates, claims jobs with conditional SQL, creates Kubernetes Jobs, and reconciles lost work. The runner executes inside each Kubernetes Job and calls the API to start, heartbeat, append logs, and complete.

PostgreSQL owns state. Redis is an optimization, not a correctness boundary. Kubernetes owns pod execution, but BuildPlane owns retries and attempt status.

The runner receives a raw lease token through environment variables. PostgreSQL stores only the token hash. Every runner callback is authenticated with the internal token and validated against job ID, attempt ID, lease token, attempt status, lease expiry, and latest attempt number.

The MVP stores logs in PostgreSQL for easy inspection. Artifacts and caches use S3-compatible object keys and metadata rows.
