# ADR 0001: PostgreSQL as Source of Truth

BuildPlane stores durable workflow, job, attempt, lease, log, artifact, and cache metadata in PostgreSQL. This gives transactional creation, idempotency, conditional claims, and recovery after Redis or scheduler loss.
