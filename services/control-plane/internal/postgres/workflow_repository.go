package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sivagirish/buildplane/services/control-plane/internal/workflows"
)

type WorkflowRepository struct {
	db *sql.DB
}

func NewWorkflowRepository(db *sql.DB) *WorkflowRepository {
	return &WorkflowRepository{db: db}
}

func (r *WorkflowRepository) CreateRun(ctx context.Context, params workflows.CreateRunParams) (workflows.Run, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return workflows.Run{}, false, fmt.Errorf("begin create workflow run: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	run, err := insertRun(ctx, tx, params)
	if err == nil {
		if err := insertAuditRecord(ctx, tx, auditRecordParams{
			WorkflowRunID: run.ID,
			EventType:     "workflow_run.created",
			ActorType:     "api",
			ActorID:       params.CorrelationID,
			Details: map[string]any{
				"workflow_name": run.WorkflowName,
				"status":        string(run.Status),
			},
		}); err != nil {
			return workflows.Run{}, false, err
		}
		nodeID, err := insertInitialNodeExecution(ctx, tx, run.ID, params.InitialNodeName)
		if err != nil {
			return workflows.Run{}, false, err
		}
		if err := insertAuditRecord(ctx, tx, auditRecordParams{
			WorkflowRunID:   run.ID,
			NodeExecutionID: nodeID,
			EventType:       "node_execution.created",
			ActorType:       "api",
			ActorID:         params.CorrelationID,
			Details: map[string]any{
				"node_name":   params.InitialNodeName,
				"worker_pool": workflows.WorkerPoolForNode(params.InitialNodeName),
				"status":      string(workflows.NodeStatusPending),
			},
		}); err != nil {
			return workflows.Run{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return workflows.Run{}, false, fmt.Errorf("commit create workflow run: %w", err)
		}
		return run, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return workflows.Run{}, false, err
	}

	run, err = getRunByIdempotencyKey(ctx, tx, params.IdempotencyKey)
	if err != nil {
		return workflows.Run{}, false, err
	}
	if run.RequestHash != params.RequestHash {
		return workflows.Run{}, false, workflows.ErrIdempotencyConflict
	}

	if err := tx.Commit(); err != nil {
		return workflows.Run{}, false, fmt.Errorf("commit replay workflow run: %w", err)
	}
	return run, false, nil
}

func (r *WorkflowRepository) GetRun(ctx context.Context, id string) (workflows.Run, error) {
	const query = `
SELECT id, workflow_name, status, input, idempotency_key, request_hash, correlation_id, traceparent, created_at, updated_at
FROM workflow_runs
WHERE id = $1`

	run, err := scanRun(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return workflows.Run{}, workflows.ErrNotFound
	}
	if err != nil {
		return workflows.Run{}, fmt.Errorf("get workflow run: %w", err)
	}
	return run, nil
}

func (r *WorkflowRepository) ListRuns(ctx context.Context, limit int) ([]workflows.Run, error) {
	if limit <= 0 {
		limit = 50
	}
	const query = `
SELECT id, workflow_name, status, input, idempotency_key, request_hash, correlation_id, traceparent, created_at, updated_at
FROM workflow_runs
ORDER BY updated_at DESC, created_at DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list workflow runs: %w", err)
	}
	defer rows.Close()

	var runs []workflows.Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow run: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan workflow runs: %w", err)
	}
	return runs, nil
}

func (r *WorkflowRepository) ListAuditRecords(ctx context.Context, workflowRunID string) ([]workflows.AuditRecord, error) {
	const query = `
SELECT id, workflow_run_id, node_execution_id, event_type, actor_type, actor_id, details, created_at
FROM audit_records
WHERE workflow_run_id = $1
ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query, workflowRunID)
	if err != nil {
		return nil, fmt.Errorf("list audit records: %w", err)
	}
	defer rows.Close()

	var records []workflows.AuditRecord
	for rows.Next() {
		var record workflows.AuditRecord
		var nodeExecutionID sql.NullString
		var details []byte
		if err := rows.Scan(
			&record.ID,
			&record.WorkflowRunID,
			&nodeExecutionID,
			&record.EventType,
			&record.ActorType,
			&record.ActorID,
			&details,
			&record.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit record: %w", err)
		}
		record.NodeExecutionID = nullString(nodeExecutionID)
		record.Details = json.RawMessage(details)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan audit records: %w", err)
	}

	return records, nil
}

func (r *WorkflowRepository) SubmitHumanDecision(ctx context.Context, params workflows.SubmitHumanDecisionParams) (workflows.HumanDecisionResult, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return workflows.HumanDecisionResult{}, false, fmt.Errorf("begin submit human decision: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	run, err := getRunForUpdate(ctx, tx, params.WorkflowRunID)
	if errors.Is(err, sql.ErrNoRows) {
		return workflows.HumanDecisionResult{}, false, workflows.ErrNotFound
	}
	if err != nil {
		return workflows.HumanDecisionResult{}, false, err
	}

	existing, found, err := getHumanDecisionByKey(ctx, tx, params.WorkflowRunID, params.DecisionKey)
	if err != nil {
		return workflows.HumanDecisionResult{}, false, err
	}
	if found {
		if existing.Decision != params.Decision || existing.ActorID != params.ActorID || existing.Reason != params.Reason {
			return workflows.HumanDecisionResult{}, false, workflows.ErrDecisionConflict
		}
		if err := tx.Commit(); err != nil {
			return workflows.HumanDecisionResult{}, false, fmt.Errorf("commit replay human decision: %w", err)
		}
		return workflows.HumanDecisionResult{Run: run, Decision: existing}, false, nil
	}

	if run.Status != workflows.StatusWaitingForHuman {
		return workflows.HumanDecisionResult{}, false, workflows.ErrWorkflowNotWaiting
	}

	decision, err := insertHumanDecision(ctx, tx, params)
	if err != nil {
		return workflows.HumanDecisionResult{}, false, err
	}
	if err := insertAuditRecord(ctx, tx, auditRecordParams{
		WorkflowRunID: run.ID,
		EventType:     "human_decision.recorded",
		ActorType:     "human",
		ActorID:       params.ActorID,
		Details: map[string]any{
			"decision_key": params.DecisionKey,
			"decision":     params.Decision,
			"reason":       params.Reason,
		},
	}); err != nil {
		return workflows.HumanDecisionResult{}, false, err
	}

	nextNodeName := ""
	switch params.Decision {
	case "approved":
		nextNodeName = workflows.NextNodeAfterApproval(run.WorkflowName)
		if nextNodeName == "" {
			return workflows.HumanDecisionResult{}, false, fmt.Errorf("%w: approval successor for %s", workflows.ErrUnknownWorkflow, run.WorkflowName)
		}
		nodeID, err := insertInitialNodeExecution(ctx, tx, run.ID, nextNodeName)
		if err != nil {
			return workflows.HumanDecisionResult{}, false, err
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE workflow_runs
SET status = 'queued',
	updated_at = now()
WHERE id = $1`, run.ID); err != nil {
			return workflows.HumanDecisionResult{}, false, fmt.Errorf("mark workflow resumed: %w", err)
		}
		run.Status = workflows.StatusQueued
		if err := insertAuditRecord(ctx, tx, auditRecordParams{
			WorkflowRunID: run.ID,
			EventType:     "workflow_run.resumed",
			ActorType:     "human",
			ActorID:       params.ActorID,
			Details: map[string]any{
				"decision_key": params.DecisionKey,
				"next_node":    nextNodeName,
			},
		}); err != nil {
			return workflows.HumanDecisionResult{}, false, err
		}
		if err := insertAuditRecord(ctx, tx, auditRecordParams{
			WorkflowRunID:   run.ID,
			NodeExecutionID: nodeID,
			EventType:       "node_execution.created",
			ActorType:       "human",
			ActorID:         params.ActorID,
			Details: map[string]any{
				"node_name":   nextNodeName,
				"worker_pool": workflows.WorkerPoolForNode(nextNodeName),
				"status":      string(workflows.NodeStatusPending),
			},
		}); err != nil {
			return workflows.HumanDecisionResult{}, false, err
		}
	case "rejected":
		if _, err := tx.ExecContext(ctx, `
UPDATE workflow_runs
SET status = 'canceled',
	updated_at = now()
WHERE id = $1`, run.ID); err != nil {
			return workflows.HumanDecisionResult{}, false, fmt.Errorf("mark workflow canceled: %w", err)
		}
		run.Status = workflows.StatusCanceled
		if err := insertAuditRecord(ctx, tx, auditRecordParams{
			WorkflowRunID: run.ID,
			EventType:     "workflow_run.canceled",
			ActorType:     "human",
			ActorID:       params.ActorID,
			Details: map[string]any{
				"decision_key": params.DecisionKey,
				"reason":       params.Reason,
			},
		}); err != nil {
			return workflows.HumanDecisionResult{}, false, err
		}
	}

	updatedRun, err := getRunByID(ctx, tx, run.ID)
	if err != nil {
		return workflows.HumanDecisionResult{}, false, err
	}

	if err := tx.Commit(); err != nil {
		return workflows.HumanDecisionResult{}, false, fmt.Errorf("commit submit human decision: %w", err)
	}

	return workflows.HumanDecisionResult{
		Run:          updatedRun,
		Decision:     decision,
		NextNodeName: nextNodeName,
	}, true, nil
}

func (r *WorkflowRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func insertRun(ctx context.Context, tx *sql.Tx, params workflows.CreateRunParams) (workflows.Run, error) {
	const query = `
INSERT INTO workflow_runs (
	id,
	workflow_name,
	status,
	input,
	idempotency_key,
	request_hash,
	correlation_id,
	traceparent
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (idempotency_key) DO NOTHING
RETURNING id, workflow_name, status, input, idempotency_key, request_hash, correlation_id, traceparent, created_at, updated_at`

	return scanRun(tx.QueryRowContext(
		ctx,
		query,
		params.ID,
		params.WorkflowName,
		string(params.Status),
		[]byte(params.Input),
		params.IdempotencyKey,
		params.RequestHash,
		params.CorrelationID,
		params.TraceParent,
	))
}

func insertInitialNodeExecution(ctx context.Context, tx *sql.Tx, workflowRunID string, nodeName string) (string, error) {
	if nodeName == "" {
		nodeName = "validate_input"
	}
	workerPool := workflows.WorkerPoolForNode(nodeName)

	nodeID, err := newID()
	if err != nil {
		return "", fmt.Errorf("generate node execution id: %w", err)
	}

	const query = `
INSERT INTO node_executions (
	id,
	workflow_run_id,
	node_name,
	worker_pool,
	status
) VALUES ($1, $2, $3, $4, $5)`

	if _, err := tx.ExecContext(ctx, query, nodeID, workflowRunID, nodeName, workerPool, string(workflows.NodeStatusPending)); err != nil {
		return "", fmt.Errorf("insert initial node execution: %w", err)
	}
	return nodeID, nil
}

func getRunByIdempotencyKey(ctx context.Context, tx *sql.Tx, idempotencyKey string) (workflows.Run, error) {
	const query = `
SELECT id, workflow_name, status, input, idempotency_key, request_hash, correlation_id, traceparent, created_at, updated_at
FROM workflow_runs
WHERE idempotency_key = $1`

	run, err := scanRun(tx.QueryRowContext(ctx, query, idempotencyKey))
	if err != nil {
		return workflows.Run{}, fmt.Errorf("get workflow run by idempotency key: %w", err)
	}
	return run, nil
}

func getRunByID(ctx context.Context, tx *sql.Tx, id string) (workflows.Run, error) {
	const query = `
SELECT id, workflow_name, status, input, idempotency_key, request_hash, correlation_id, traceparent, created_at, updated_at
FROM workflow_runs
WHERE id = $1`

	run, err := scanRun(tx.QueryRowContext(ctx, query, id))
	if err != nil {
		return workflows.Run{}, fmt.Errorf("get workflow run by id: %w", err)
	}
	return run, nil
}

func getRunForUpdate(ctx context.Context, tx *sql.Tx, id string) (workflows.Run, error) {
	const query = `
SELECT id, workflow_name, status, input, idempotency_key, request_hash, correlation_id, traceparent, created_at, updated_at
FROM workflow_runs
WHERE id = $1
FOR UPDATE`

	return scanRun(tx.QueryRowContext(ctx, query, id))
}

func insertHumanDecision(ctx context.Context, tx *sql.Tx, params workflows.SubmitHumanDecisionParams) (workflows.HumanDecision, error) {
	const query = `
INSERT INTO human_decisions (
	id,
	workflow_run_id,
	decision_key,
	node_name,
	decision,
	actor_id,
	reason,
	details
) VALUES ($1, $2, $3, 'await_human_approval', $4, $5, $6, '{}'::jsonb)
RETURNING id, workflow_run_id, decision_key, node_name, decision, actor_id, reason, details, created_at`

	decision, err := scanHumanDecision(tx.QueryRowContext(
		ctx,
		query,
		params.ID,
		params.WorkflowRunID,
		params.DecisionKey,
		params.Decision,
		params.ActorID,
		params.Reason,
	))
	if err != nil {
		return workflows.HumanDecision{}, fmt.Errorf("insert human decision: %w", err)
	}
	return decision, nil
}

func getHumanDecisionByKey(ctx context.Context, tx *sql.Tx, workflowRunID string, decisionKey string) (workflows.HumanDecision, bool, error) {
	const query = `
SELECT id, workflow_run_id, decision_key, node_name, decision, actor_id, reason, details, created_at
FROM human_decisions
WHERE workflow_run_id = $1
	AND decision_key = $2`

	decision, err := scanHumanDecision(tx.QueryRowContext(ctx, query, workflowRunID, decisionKey))
	if errors.Is(err, sql.ErrNoRows) {
		return workflows.HumanDecision{}, false, nil
	}
	if err != nil {
		return workflows.HumanDecision{}, false, fmt.Errorf("get human decision by key: %w", err)
	}
	return decision, true, nil
}

func scanHumanDecision(row rowScanner) (workflows.HumanDecision, error) {
	var decision workflows.HumanDecision
	var details []byte
	if err := row.Scan(
		&decision.ID,
		&decision.WorkflowRunID,
		&decision.DecisionKey,
		&decision.NodeName,
		&decision.Decision,
		&decision.ActorID,
		&decision.Reason,
		&details,
		&decision.CreatedAt,
	); err != nil {
		return workflows.HumanDecision{}, err
	}
	decision.Details = json.RawMessage(details)
	return decision, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRun(row rowScanner) (workflows.Run, error) {
	var run workflows.Run
	var status string
	var inputBytes []byte

	if err := row.Scan(
		&run.ID,
		&run.WorkflowName,
		&status,
		&inputBytes,
		&run.IdempotencyKey,
		&run.RequestHash,
		&run.CorrelationID,
		&run.TraceParent,
		&run.CreatedAt,
		&run.UpdatedAt,
	); err != nil {
		return workflows.Run{}, err
	}

	run.Status = workflows.Status(status)
	run.Input = json.RawMessage(inputBytes)
	return run, nil
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func nullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullTime(value sql.NullTime) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}
