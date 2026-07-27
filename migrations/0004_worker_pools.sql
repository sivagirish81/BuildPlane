ALTER TABLE node_executions
ADD COLUMN worker_pool text NOT NULL DEFAULT 'general';

UPDATE node_executions
SET worker_pool = CASE
	WHEN node_name = 'classify_issue' THEN 'ai'
	ELSE 'general'
END;

ALTER TABLE node_executions
ADD CONSTRAINT node_executions_worker_pool_check
CHECK (worker_pool IN ('general', 'ai'));

CREATE INDEX idx_node_executions_schedulable_pool
ON node_executions (worker_pool, status, created_at);
