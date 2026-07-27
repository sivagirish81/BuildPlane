ALTER TABLE workflow_runs
ADD COLUMN traceparent text NOT NULL DEFAULT '';

