CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS tenants (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  scheduling_weight INTEGER NOT NULL DEFAULT 1 CHECK (scheduling_weight > 0),
  max_concurrent_jobs INTEGER NOT NULL DEFAULT 2 CHECK (max_concurrent_jobs > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workflows (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id TEXT NOT NULL REFERENCES tenants(id),
  name TEXT NOT NULL,
  definition JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workflow_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id UUID NOT NULL REFERENCES workflows(id),
  tenant_id TEXT NOT NULL REFERENCES tenants(id),
  repository_url TEXT NOT NULL,
  commit_sha TEXT NOT NULL,
  status TEXT NOT NULL,
  priority TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  version INTEGER NOT NULL DEFAULT 1,
  UNIQUE (tenant_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS jobs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_run_id UUID NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
  tenant_id TEXT NOT NULL REFERENCES tenants(id),
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  priority TEXT NOT NULL,
  commands JSONB NOT NULL,
  cache JSONB NOT NULL DEFAULT '{}',
  requested_cpu_millis INTEGER NOT NULL,
  requested_memory_mb INTEGER NOT NULL,
  max_attempts INTEGER NOT NULL,
  attempt_count INTEGER NOT NULL DEFAULT 0,
  queued_at TIMESTAMPTZ,
  scheduled_at TIMESTAMPTZ,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  version INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS jobs_status_priority_idx ON jobs(status, priority, queued_at);
CREATE INDEX IF NOT EXISTS jobs_tenant_status_idx ON jobs(tenant_id, status);
CREATE INDEX IF NOT EXISTS jobs_run_idx ON jobs(workflow_run_id);

CREATE TABLE IF NOT EXISTS job_attempts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
  attempt_number INTEGER NOT NULL,
  status TEXT NOT NULL,
  kubernetes_job_name TEXT NOT NULL,
  lease_token_hash TEXT NOT NULL,
  lease_expires_at TIMESTAMPTZ NOT NULL,
  last_heartbeat_at TIMESTAMPTZ,
  exit_code INTEGER,
  failure_reason TEXT,
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  UNIQUE(job_id, attempt_number)
);

CREATE INDEX IF NOT EXISTS attempts_active_idx ON job_attempts(status, lease_expires_at);
CREATE INDEX IF NOT EXISTS attempts_job_idx ON job_attempts(job_id, attempt_number DESC);

CREATE TABLE IF NOT EXISTS job_logs (
  id BIGSERIAL PRIMARY KEY,
  job_attempt_id UUID NOT NULL REFERENCES job_attempts(id) ON DELETE CASCADE,
  chunk TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS artifacts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  job_attempt_id UUID NOT NULL REFERENCES job_attempts(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  object_key TEXT NOT NULL,
  checksum TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS cache_entries (
  cache_key TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL REFERENCES tenants(id),
  object_key TEXT NOT NULL,
  checksum TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_accessed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ
);

INSERT INTO tenants(id, name, scheduling_weight, max_concurrent_jobs)
VALUES
  ('tenant-a', 'Tenant A', 1, 2),
  ('tenant-b', 'Tenant B', 3, 4)
ON CONFLICT (id) DO NOTHING;
