# Interview Guide

## 30 seconds

BuildPlane is a focused CI orchestration platform. It accepts workflow runs, persists durable state in PostgreSQL, uses Redis as a derived priority queue, fairly schedules across tenants, executes attempts as Kubernetes Jobs, and recovers failures with leases and reconciliation.

## 3 minutes

The API owns submission, idempotency, status, logs, and runner callbacks. PostgreSQL is the source of truth. Redis accelerates ready-job discovery but can be rebuilt. The scheduler enforces global and tenant limits, uses weighted priority plus deficit round-robin, claims jobs atomically, creates Kubernetes Jobs, and reconciles expired leases. The runner clones a repository, executes commands, sends heartbeats, uploads logs, and reports completion. Stale results cannot overwrite newer attempts because callbacks must prove they own the active lease.

## 15-minute deep dive

Discuss the state machine, why Redis is derived, why attempts are separate from jobs, how lease tokens prevent stale updates, why Kubernetes retries are disabled, how deficit round-robin handles noisy tenants, how cache keys avoid false hits, and how queue-driven autoscaling differs from CPU HPA.

## Hardest Decisions

- At-least-once execution instead of pretending exactly once.
- PostgreSQL as durable state with Redis as disposable acceleration.
- Kubernetes Jobs per attempt for isolation and operational familiarity.
- Lease-based attempt ownership to reject stale callbacks.
- Deficit round-robin to balance priority and tenant fairness.

## Likely Questions

1. Why not only Redis Streams? PostgreSQL gives transactional run/job/attempt state and easier recovery.
2. What happens if Redis loses data? Reconciliation rebuilds queues from `QUEUED` jobs.
3. Can a job execute twice? Yes, at-least-once means duplicate execution is possible.
4. How do stale completions get rejected? Latest attempt, active status, token hash, and lease expiry are checked.
5. Why disable Kubernetes Job retries? BuildPlane owns retry classification and attempt records.
6. How is tenant fairness enforced? Deficit round-robin with tenant weights and CPU-based cost.
7. How does priority differ from tenant weight? Priority is urgency class; tenant weight is fair share.
8. How are large jobs handled? They accrue deficit until cost is covered.
9. Can low priority starve? Weighted cycling gives it periodic service.
10. How do you scale to one million jobs per day? Partition queues, shard tenants, batch claims, move logs to object storage, and separate scheduler replicas by queue partition.
11. How would multi-region work? Regional schedulers with local execution, global control plane, replicated metadata, and region-aware run placement.
12. How do you secure untrusted builds? Dedicated nodes, microVMs, network policy, egress controls, secret isolation, image allowlists, and workload identity.
13. Why not use GitHub Actions directly? BuildPlane demonstrates the orchestration substrate and custom scheduling/recovery concerns.
14. What is the source of truth? PostgreSQL.
15. What consistency model is claimed? Strong state transitions, eventual queue/Kubernetes reconciliation.
16. How are retries classified? Infrastructure errors retry; user command failures do not.
17. How is idempotency implemented? Unique tenant/idempotency-key and lookup before insert.
18. What if the scheduler crashes after claiming? Reconciler detects stale scheduling/lease state.
19. What if the runner cannot reach the API? Lease expires and work retries.
20. What metrics matter first? Queue age, active jobs, retries, lease expirations, stale updates, and duration.
21. Why queue age over depth? A shallow but old queue indicates insufficient service.
22. What is cache poisoning? A less-trusted job producing cache consumed by a more-trusted job.
23. How are cache keys built? Normalized inputs hashed with SHA-256.
24. Where should logs live in production? Object storage with indexed chunks.
25. How do you cancel active jobs? Mark DB state and delete Kubernetes Job.
26. Can cancellation race completion? Conditional updates decide one winner.
27. How do you prevent scheduler bottlenecks? Partition queues and use advisory ownership per partition.
28. How are resources admitted? Active counts and in-flight CPU/memory limits.
29. What is shadow scheduling? Canary computes a decision without executing and logs disagreement.
30. How do you test fairness? Deterministic candidate sets and long-run allocation assertions.
31. How do you test lease safety? Expire an old attempt and submit a late completion.
32. Why store attempt rows? Auditing, retries, and stale-result rejection.
33. How do you handle private repos? Workload identity, short-lived credentials, and secret mounts.
34. What are operational dashboards? Queue, fairness, reliability, cache, autoscaling.
35. Why Kubernetes Jobs? Native isolation, deadlines, resource controls, and logs.
36. What is the biggest MVP weakness? Shared-cluster security for untrusted code.
37. How do you reduce DB load? Batch queue snapshots, materialized counters, partitioning.
38. How do you handle PostgreSQL outage? API rejects or buffers minimally; scheduler pauses.
39. How are artifacts handled? Object keys with metadata and checksums.
40. What would you build next? Full Kubernetes reconciliation, object-backed logs, cache runner integration, and partitioned schedulers.
