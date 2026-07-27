CREATE TABLE workflow_runs (
	id text PRIMARY KEY,
	workflow_name text NOT NULL,
	status text NOT NULL,
	input jsonb NOT NULL,
	idempotency_key text NOT NULL UNIQUE,
	request_hash text NOT NULL,
	correlation_id text NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT workflow_runs_status_check CHECK (
		status IN ('queued', 'running', 'succeeded', 'failed', 'canceled')
	)
);

CREATE INDEX workflow_runs_status_created_at_idx
	ON workflow_runs (status, created_at);
