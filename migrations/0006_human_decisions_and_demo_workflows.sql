ALTER TABLE workflow_runs
DROP CONSTRAINT workflow_runs_status_check;

ALTER TABLE workflow_runs
ADD CONSTRAINT workflow_runs_status_check CHECK (
	status IN ('queued', 'running', 'waiting_for_human', 'succeeded', 'failed', 'canceled')
);

CREATE TABLE human_decisions (
	id text PRIMARY KEY,
	workflow_run_id text NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
	decision_key text NOT NULL,
	node_name text NOT NULL,
	decision text NOT NULL,
	actor_id text NOT NULL,
	reason text NOT NULL DEFAULT '',
	details jsonb NOT NULL DEFAULT '{}'::jsonb,
	created_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT human_decisions_decision_check CHECK (
		decision IN ('approved', 'rejected')
	),
	CONSTRAINT human_decisions_unique_key UNIQUE (workflow_run_id, decision_key)
);

CREATE INDEX human_decisions_workflow_run_id_created_at_idx
	ON human_decisions (workflow_run_id, created_at);
