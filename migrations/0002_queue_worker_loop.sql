CREATE TABLE node_executions (
	id text PRIMARY KEY,
	workflow_run_id text NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
	node_name text NOT NULL,
	status text NOT NULL,
	attempt integer NOT NULL DEFAULT 0,
	lease_worker_id text,
	lease_expires_at timestamptz,
	fencing_token bigint NOT NULL DEFAULT 0,
	result jsonb,
	error text,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT node_executions_status_check CHECK (
		status IN ('pending', 'queued', 'running', 'succeeded', 'failed', 'canceled')
	)
);

CREATE INDEX node_executions_schedulable_idx
	ON node_executions (status, lease_expires_at, created_at);

CREATE INDEX node_executions_workflow_run_id_idx
	ON node_executions (workflow_run_id);

CREATE TABLE outbox_events (
	id bigserial PRIMARY KEY,
	topic text NOT NULL,
	payload jsonb NOT NULL,
	status text NOT NULL DEFAULT 'pending',
	attempts integer NOT NULL DEFAULT 0,
	available_at timestamptz NOT NULL DEFAULT now(),
	published_at timestamptz,
	last_error text,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT outbox_events_status_check CHECK (
		status IN ('pending', 'published', 'failed')
	)
);

CREATE INDEX outbox_events_publishable_idx
	ON outbox_events (status, available_at, id);
