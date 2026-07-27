package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sivagirish/buildplane/services/control-plane/internal/workflows"
)

func (r *WorkflowRepository) ClaimSchedulableNodeExecutions(ctx context.Context, limit int) ([]workflows.OutboxEvent, error) {
	if limit <= 0 {
		limit = 10
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin claim node executions: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const query = `
WITH candidates AS (
	SELECT id
	FROM node_executions
	WHERE status = 'pending'
		OR (status = 'running' AND lease_expires_at < now())
	ORDER BY created_at
	LIMIT $1
	FOR UPDATE SKIP LOCKED
),
updated AS (
	UPDATE node_executions AS n
	SET status = 'queued',
		lease_worker_id = NULL,
		lease_expires_at = NULL,
		updated_at = now()
	FROM candidates AS c
	WHERE n.id = c.id
	RETURNING n.id, n.workflow_run_id, n.node_name
),
events AS (
	INSERT INTO outbox_events (topic, payload)
	SELECT
		'node_execution.queued',
		jsonb_build_object(
			'node_execution_id', id,
			'workflow_run_id', workflow_run_id,
			'node_name', node_name
		)
	FROM updated
	RETURNING id, topic, payload, status, attempts, published_at, last_error, created_at, updated_at
)
SELECT id, topic, payload, status, attempts, published_at, last_error, created_at, updated_at
FROM events`

	rows, err := tx.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("claim node executions: %w", err)
	}
	defer rows.Close()

	events, err := scanOutboxEvents(rows)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit claim node executions: %w", err)
	}

	return events, nil
}

func (r *WorkflowRepository) ListPublishableOutboxEvents(ctx context.Context, limit int) ([]workflows.OutboxEvent, error) {
	if limit <= 0 {
		limit = 10
	}

	const query = `
SELECT id, topic, payload, status, attempts, published_at, last_error, created_at, updated_at
FROM outbox_events
WHERE status IN ('pending', 'failed')
	AND available_at <= now()
ORDER BY id
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list publishable outbox events: %w", err)
	}
	defer rows.Close()

	return scanOutboxEvents(rows)
}

func (r *WorkflowRepository) MarkOutboxPublished(ctx context.Context, id int64) error {
	const query = `
UPDATE outbox_events
SET status = 'published',
	published_at = now(),
	updated_at = now(),
	last_error = NULL
WHERE id = $1`

	if _, err := r.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("mark outbox published: %w", err)
	}
	return nil
}

func (r *WorkflowRepository) MarkOutboxFailed(ctx context.Context, id int64, errText string) error {
	const query = `
UPDATE outbox_events
SET status = 'failed',
	attempts = attempts + 1,
	available_at = now() + interval '5 seconds',
	last_error = $2,
	updated_at = now()
WHERE id = $1`

	if _, err := r.db.ExecContext(ctx, query, id, errText); err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}
	return nil
}

func (r *WorkflowRepository) AcquireNodeExecutionLease(ctx context.Context, nodeExecutionID string, workerID string, leaseDuration time.Duration) (workflows.Lease, error) {
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return workflows.Lease{}, fmt.Errorf("begin acquire lease: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const query = `
UPDATE node_executions
SET status = 'running',
	attempt = attempt + 1,
	lease_worker_id = $2,
	lease_expires_at = now() + ($3::text || ' milliseconds')::interval,
	fencing_token = fencing_token + 1,
	updated_at = now()
WHERE id = $1
	AND (
		status = 'queued'
		OR (status = 'running' AND lease_expires_at < now())
	)
RETURNING id, workflow_run_id, node_name, attempt, lease_worker_id, lease_expires_at, fencing_token`

	var lease workflows.Lease
	err = tx.QueryRowContext(ctx, query, nodeExecutionID, workerID, leaseDuration.Milliseconds()).Scan(
		&lease.NodeExecutionID,
		&lease.WorkflowRunID,
		&lease.NodeName,
		&lease.Attempt,
		&lease.WorkerID,
		&lease.LeaseExpiresAt,
		&lease.FencingToken,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return workflows.Lease{}, workflows.ErrLeaseUnavailable
	}
	if err != nil {
		return workflows.Lease{}, fmt.Errorf("acquire node execution lease: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE workflow_runs
SET status = 'running',
	updated_at = now()
WHERE id = $1
	AND status = 'queued'`, lease.WorkflowRunID); err != nil {
		return workflows.Lease{}, fmt.Errorf("mark workflow running: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return workflows.Lease{}, fmt.Errorf("commit acquire lease: %w", err)
	}

	return lease, nil
}

func (r *WorkflowRepository) HeartbeatNodeExecution(ctx context.Context, nodeExecutionID string, workerID string, fencingToken int64, leaseDuration time.Duration) error {
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}

	const query = `
UPDATE node_executions
SET lease_expires_at = now() + ($4::text || ' milliseconds')::interval,
	updated_at = now()
WHERE id = $1
	AND lease_worker_id = $2
	AND fencing_token = $3
	AND status = 'running'`

	result, err := r.db.ExecContext(ctx, query, nodeExecutionID, workerID, fencingToken, leaseDuration.Milliseconds())
	if err != nil {
		return fmt.Errorf("heartbeat node execution: %w", err)
	}
	return requireOneRow(result, workflows.ErrStaleLease)
}

func (r *WorkflowRepository) CompleteNodeExecution(ctx context.Context, nodeExecutionID string, workerID string, fencingToken int64, result json.RawMessage) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin complete node execution: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const query = `
UPDATE node_executions
SET status = 'succeeded',
	result = $4,
	error = NULL,
	lease_expires_at = NULL,
	updated_at = now()
WHERE id = $1
	AND lease_worker_id = $2
	AND fencing_token = $3
	AND status = 'running'
RETURNING workflow_run_id`

	var workflowRunID string
	if err := tx.QueryRowContext(ctx, query, nodeExecutionID, workerID, fencingToken, []byte(result)).Scan(&workflowRunID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return completeAlreadySucceeded(ctx, tx, nodeExecutionID, workerID, fencingToken)
		}
		return fmt.Errorf("complete node execution: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE workflow_runs
SET status = 'succeeded',
	updated_at = now()
WHERE id = $1
	AND NOT EXISTS (
		SELECT 1
		FROM node_executions
		WHERE workflow_run_id = $1
			AND status NOT IN ('succeeded')
	)`, workflowRunID); err != nil {
		return fmt.Errorf("mark workflow succeeded: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit complete node execution: %w", err)
	}

	return nil
}

func completeAlreadySucceeded(ctx context.Context, tx *sql.Tx, nodeExecutionID string, workerID string, fencingToken int64) error {
	const query = `
SELECT EXISTS (
	SELECT 1
	FROM node_executions
	WHERE id = $1
		AND lease_worker_id = $2
		AND fencing_token = $3
		AND status = 'succeeded'
)`

	var exists bool
	if err := tx.QueryRowContext(ctx, query, nodeExecutionID, workerID, fencingToken).Scan(&exists); err != nil {
		return fmt.Errorf("check completed node execution: %w", err)
	}
	if exists {
		return tx.Commit()
	}
	return workflows.ErrStaleLease
}

func (r *WorkflowRepository) FailNodeExecution(ctx context.Context, nodeExecutionID string, workerID string, fencingToken int64, errText string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin fail node execution: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const query = `
UPDATE node_executions
SET status = 'failed',
	error = $4,
	lease_expires_at = NULL,
	updated_at = now()
WHERE id = $1
	AND lease_worker_id = $2
	AND fencing_token = $3
	AND status = 'running'
RETURNING workflow_run_id`

	var workflowRunID string
	if err := tx.QueryRowContext(ctx, query, nodeExecutionID, workerID, fencingToken, errText).Scan(&workflowRunID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return workflows.ErrStaleLease
		}
		return fmt.Errorf("fail node execution: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE workflow_runs
SET status = 'failed',
	updated_at = now()
WHERE id = $1`, workflowRunID); err != nil {
		return fmt.Errorf("mark workflow failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit fail node execution: %w", err)
	}

	return nil
}

func scanOutboxEvents(rows *sql.Rows) ([]workflows.OutboxEvent, error) {
	var events []workflows.OutboxEvent
	for rows.Next() {
		var event workflows.OutboxEvent
		var payload []byte
		var publishedAt sql.NullTime
		var lastError sql.NullString
		if err := rows.Scan(
			&event.ID,
			&event.Topic,
			&payload,
			&event.Status,
			&event.Attempts,
			&publishedAt,
			&lastError,
			&event.CreatedAt,
			&event.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		event.Payload = json.RawMessage(payload)
		event.PublishedAt = nullTime(publishedAt)
		event.LastError = nullString(lastError)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan outbox events: %w", err)
	}
	return events, nil
}

func requireOneRow(result sql.Result, errIfZero error) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read rows affected: %w", err)
	}
	if affected == 0 {
		return errIfZero
	}
	return nil
}
