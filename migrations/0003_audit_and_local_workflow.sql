CREATE TABLE audit_records (
	id bigserial PRIMARY KEY,
	workflow_run_id text NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
	node_execution_id text REFERENCES node_executions(id) ON DELETE SET NULL,
	event_type text NOT NULL,
	actor_type text NOT NULL,
	actor_id text NOT NULL,
	details jsonb NOT NULL DEFAULT '{}'::jsonb,
	created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_records_workflow_run_id_id_idx
	ON audit_records (workflow_run_id, id);

CREATE INDEX audit_records_event_type_idx
	ON audit_records (event_type);
